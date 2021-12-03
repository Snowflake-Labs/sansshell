package client

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/subcommands"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

func init() {
	subcommands.Register(&readCmd{}, "file")
	subcommands.Register(&tailCmd{}, "file")
	subcommands.Register(&statCmd{}, "file")
	subcommands.Register(&sumCmd{}, "file")
	subcommands.Register(&chownCmd{}, "file")
	subcommands.Register(&chgrpCmd{}, "file")
	subcommands.Register(&chmodCmd{}, "file")
	subcommands.Register(&immutableCmd{}, "file")
	subcommands.Register(&lsCmd{}, "file")
}

type readCmd struct {
	offset int64
	length int64
}

func (*readCmd) Name() string     { return "read" }
func (*readCmd) Synopsis() string { return "Read a file." }
func (*readCmd) Usage() string {
	return `read <path>:
  Read from the remote file named by <path> and write it to the appropriate --output destination.
`
}

func (p *readCmd) SetFlags(f *flag.FlagSet) {
	f.Int64Var(&p.offset, "offset", 0, "If positive bytes to skip before reading. If negative apply from the end of the file")
	f.Int64Var(&p.length, "length", 0, "If positive the maximum number of bytes to read")
}

func (p *readCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Please specify a filename to read.")
		return subcommands.ExitUsageError
	}

	filename := f.Args()[0]
	req := &pb.ReadActionRequest{
		Request: &pb.ReadActionRequest_File{
			File: &pb.ReadRequest{
				Filename: filename,
				Offset:   p.offset,
				Length:   p.length,
			},
		},
	}

	err := ReadFile(ctx, state, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read file %s: %v\n", filename, err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

func ReadFile(ctx context.Context, state *util.ExecuteState, req *pb.ReadActionRequest) error {
	c := pb.NewLocalFileClientProxy(state.Conn)
	stream, err := c.ReadOneMany(ctx, req)
	if err != nil {
		return err
	}

	var retErr error
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			retErr = err
			break
		}
		for _, r := range resp {
			contents := r.Resp.Contents
			if r.Error != nil && r.Error != io.EOF {
				contents = []byte(fmt.Sprintf("Target %s (%d) returned error - %v", r.Target, r.Index, r.Error))
				retErr = errors.New("error received for remote file. See output for details")
			}
			n, err := state.Out[r.Index].Write(contents)
			if got, want := n, len(contents); got != want {
				return fmt.Errorf("can't write into buffer at correct length. Got %d want %d", got, want)
			}
			if err != nil {
				return fmt.Errorf("error writing output to output index %d - %v", r.Index, err)
			}
		}
	}
	return retErr
}

type tailCmd struct {
	offset int64
}

func (*tailCmd) Name() string     { return "tail" }
func (*tailCmd) Synopsis() string { return "Tail a file." }
func (*tailCmd) Usage() string {
	return `tail <path>:
  Tail the remote file named by <path> and write it to the appropriate --output destination. This
  will continue to block and read until cancelled (as tail -f would do locally).
`
}

func (p *tailCmd) SetFlags(f *flag.FlagSet) {
	f.Int64Var(&p.offset, "offset", 0, "If positive bytes to skip before reading. If negative apply from the end of the file")
}

func (p *tailCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Please specify a filename to tail.")
		return subcommands.ExitUsageError
	}

	filename := f.Args()[0]
	req := &pb.ReadActionRequest{
		Request: &pb.ReadActionRequest_Tail{
			Tail: &pb.TailRequest{
				Filename: filename,
				Offset:   p.offset,
			},
		},
	}

	err := ReadFile(ctx, state, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not tail file: %v\n", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

type statCmd struct{}

func (*statCmd) Name() string     { return "stat" }
func (*statCmd) Synopsis() string { return "Stat file(s) to stdout." }
func (*statCmd) Usage() string {
	return `stat <path> [<path>...]:
  Stat one or more remote fles and print the result to stdout.
  `
}

func (*statCmd) SetFlags(f *flag.FlagSet) {}

func (s *statCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)

	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify at least one path to stat")
		return subcommands.ExitUsageError
	}
	client := pb.NewLocalFileClientProxy(state.Conn)
	stream, err := client.StatOneMany(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat client error: %v\n", err)
		return subcommands.ExitFailure
	}

	waitc := make(chan error)

	// Push all the sends into their own routine so we're receiving replies
	// at the same time. This way we can't deadlock if we send so much this
	// blocks which makes the server block its sends waiting on us to receive.
	go func() {
		var err error
		for _, filename := range f.Args() {
			if err = stream.Send(&pb.StatRequest{Filename: filename}); err != nil {
				fmt.Fprintf(os.Stderr, "stat: send error: %v\n", err)
				break
			}
		}
		// Close the sending stream to notify the server not to expect any further data.
		// We'll process below but this let's the server politely know we're done sending
		// as otherwise it'll see this as a cancellation.
		stream.CloseSend()
		waitc <- err
		close(waitc)
	}()

	retCode := subcommands.ExitSuccess
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "stat: receive error: %v\n", err)
			return subcommands.ExitFailure
		}
		for _, r := range reply {
			if r.Error != nil {
				if r.Error != io.EOF {
					fmt.Fprintf(state.Out[r.Index], "Got error from target %s (%d) - %v\n", r.Target, r.Index, r.Error)
					retCode = subcommands.ExitFailure
				}
				continue
			}
			mode := os.FileMode(r.Resp.Mode)
			outTmpl := "File: %s\nSize: %d\nType: %s\nAccess: %s Uid: %d Gid: %d\nModify: %s\nImmutable: %t\n"
			fmt.Fprintf(state.Out[r.Index], outTmpl, r.Resp.Filename, r.Resp.Size, fileTypeString(mode), mode, r.Resp.Uid, r.Resp.Gid, r.Resp.Modtime.AsTime(), r.Resp.Immutable)
		}
	}
	for err := range waitc {
		if err != nil {
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}

func fileTypeString(mode os.FileMode) string {
	typ := mode.Type()
	switch {
	case typ&fs.ModeDir != 0:
		return "directory"
	case typ&fs.ModeSymlink != 0:
		return "symbolic link"
	case typ&fs.ModeNamedPipe != 0:
		return "FIFO"
	case typ&fs.ModeSocket != 0:
		return "socket"
	case typ&fs.ModeCharDevice != 0:
		return "character device"
	case typ&fs.ModeDevice != 0:
		return "block device"
	case typ&fs.ModeIrregular != 0:
		return "irregular file"
	case typ&fs.ModeType == 0:
		return "regular file"
	default:
		return "UNKNOWN"
	}
}

type sumCmd struct {
	sumType string
}

func (*sumCmd) Name() string     { return "sum" }
func (*sumCmd) Synopsis() string { return "Calculate file sums" }
func (*sumCmd) Usage() string {
	return `sum [--sumtype type ] <path> [<path>...]
  Calculate sums for one or more remotes paths and print the result (as a hexidecimal string) to stdout.
  `
}

func flagToType(val string) (pb.SumType, error) {
	v := fmt.Sprintf("SUM_TYPE_%s", strings.ToUpper(val))
	i, ok := pb.SumType_value[v]
	if !ok {
		return pb.SumType_SUM_TYPE_UNKNOWN, fmt.Errorf("no such sumtype value: %s", v)
	}
	return pb.SumType(i), nil
}

func (s *sumCmd) SetFlags(f *flag.FlagSet) {
	var shortNames []string
	for k := range pb.SumType_value {
		shortNames = append(shortNames, strings.TrimPrefix(k, "SUM_TYPE_"))
	}
	sort.Strings(shortNames)
	f.StringVar(&s.sumType, "sumtype", "SHA256", fmt.Sprintf("Type of sum to calculate (one of: [%s])", strings.Join(shortNames, ",")))
}

func (s *sumCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify a filename to sum")
		return subcommands.ExitUsageError
	}
	sumType, err := flagToType(s.sumType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flag error: %v\n", err)
		return subcommands.ExitUsageError
	}
	client := pb.NewLocalFileClientProxy(state.Conn)
	stream, err := client.SumOneMany(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sum client error: %v\n", err)
		return subcommands.ExitFailure
	}

	waitc := make(chan error)

	// Push all the sends into their own routine so we're receiving replies
	// at the same time. This way we can't deadlock if we send so much this
	// blocks which makes the server block its sends waiting on us to receive.
	go func() {
		var err error
		for _, filename := range f.Args() {
			if err = stream.Send(&pb.SumRequest{Filename: filename, SumType: sumType}); err != nil {
				fmt.Fprintf(os.Stderr, "sum: send error: %v\n", err)
				break
			}
		}
		// Close the sending stream to notify the server not to expect any further data.
		// We'll process below but this let's the server politely know we're done sending
		// as otherwise it'll see this as a cancellation.
		stream.CloseSend()
		waitc <- err
		close(waitc)
	}()

	retCode := subcommands.ExitSuccess
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "stat: receive error: %v\n", err)
			retCode = subcommands.ExitFailure
			break
		}
		for _, r := range reply {
			if r.Error != nil {
				if r.Error != io.EOF {
					fmt.Fprintf(state.Out[r.Index], "Got error from target %s (%d) - %v\n", r.Target, r.Index, r.Error)
					retCode = subcommands.ExitFailure
				}
				continue
			}
			fmt.Fprintf(state.Out[r.Index], "%s %s\n", r.Resp.Sum, r.Resp.Filename)
		}
	}

	for err := range waitc {
		if err != nil {
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}

type chownCmd struct {
	uid int
}

func (*chownCmd) Name() string     { return "chown" }
func (*chownCmd) Synopsis() string { return "Change ownership on file/directory" }
func (*chownCmd) Usage() string {
	return `chown --uid=X <path>:
  Change ownership on a file/directory.
  `
}

func (c *chownCmd) SetFlags(f *flag.FlagSet) {
	f.IntVar(&c.uid, "uid", -1, "Sets the file/directory to be owned by this uid")
}

func (c *chownCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify a filename to chown")
		return subcommands.ExitUsageError
	}
	if c.uid < 0 {
		fmt.Fprintln(os.Stderr, "--uid must be set to a positive number")
		return subcommands.ExitFailure
	}

	req := &pb.SetFileAttributesRequest{
		Attrs: &pb.FileAttributes{
			Filename: f.Args()[0],
			Attributes: []*pb.FileAttribute{
				{
					Value: &pb.FileAttribute_Uid{
						Uid: uint32(c.uid),
					},
				},
			},
		},
	}

	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.SetFileAttributesOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chown client error: %v\n", err)
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(os.Stderr, "chown client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

type chgrpCmd struct {
	gid int
}

func (*chgrpCmd) Name() string     { return "chgrp" }
func (*chgrpCmd) Synopsis() string { return "Change group membership on a file/directory" }
func (*chgrpCmd) Usage() string {
	return `chgrp --gid=X <path>:
  Change group membership on a file/directory.
  `
}

func (c *chgrpCmd) SetFlags(f *flag.FlagSet) {
	f.IntVar(&c.gid, "gid", -1, "Sets the file/directory group membership specified by this gid")

}

func (c *chgrpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify a filename to chgrp")
		return subcommands.ExitUsageError
	}
	if c.gid < 0 {
		fmt.Fprintln(os.Stderr, "--gid must be set to a positive number")
		return subcommands.ExitFailure
	}

	req := &pb.SetFileAttributesRequest{
		Attrs: &pb.FileAttributes{
			Filename: f.Args()[0],
			Attributes: []*pb.FileAttribute{
				{
					Value: &pb.FileAttribute_Gid{
						Gid: uint32(c.gid),
					},
				},
			},
		},
	}

	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.SetFileAttributesOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chgrp client error: %v\n", err)
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(os.Stderr, "chgrp client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

type chmodCmd struct {
	mode uint64
}

func (*chmodCmd) Name() string     { return "chmod" }
func (*chmodCmd) Synopsis() string { return "Change mode on a file/directory" }
func (*chmodCmd) Usage() string {
	return `chmod --mode=X <path>:
  Change the modes on a file/directory.
  `
}

func (c *chmodCmd) SetFlags(f *flag.FlagSet) {
	f.Uint64Var(&c.mode, "mode", 0, "Sets the file/directory to this mode")
}

func (c *chmodCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify a filename to chmod")
		return subcommands.ExitUsageError
	}
	if c.mode == 0 {
		fmt.Fprintln(os.Stderr, "--mode must be set to a non-zero value")
		return subcommands.ExitFailure
	}

	req := &pb.SetFileAttributesRequest{
		Attrs: &pb.FileAttributes{
			Filename: f.Args()[0],
			Attributes: []*pb.FileAttribute{
				{
					Value: &pb.FileAttribute_Mode{
						Mode: uint32(c.mode),
					},
				},
			},
		},
	}

	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.SetFileAttributesOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chmod client error: %v\n", err)
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(os.Stderr, "chmod client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

type immutableCmd struct {
	immutable bool
}

func (*immutableCmd) Name() string     { return "immutable" }
func (*immutableCmd) Synopsis() string { return "Set or clear the immutable bit on a file/directory." }
func (*immutableCmd) Usage() string {
	return `immutable --state=X <path>:
  Set or clears the immutable bit on a file/directory.
  `
}

func (i *immutableCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&i.immutable, "state", false, "Sets or clears the immutable bit on a file/directory")
}

func (i *immutableCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify a filename to chmod")
		return subcommands.ExitUsageError
	}

	req := &pb.SetFileAttributesRequest{
		Attrs: &pb.FileAttributes{
			Filename: f.Args()[0],
			Attributes: []*pb.FileAttribute{
				{
					Value: &pb.FileAttribute_Immutable{
						Immutable: i.immutable,
					},
				},
			},
		},
	}
	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.SetFileAttributesOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "immutable client error: %v\n", err)
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(os.Stderr, "immutable client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

type lsCmd struct {
	long      bool
	directory bool
}

func (*lsCmd) Name() string     { return "ls" }
func (*lsCmd) Synopsis() string { return "List a file or directory" }
func (*lsCmd) Usage() string {
	return `ls [--long] [--directory] <path>:
  List the path given printing out each entry (N if it's a directory). Use --long to get ls -l style output.
  If the entry is a directory it will be suppressed from the output unless --directory is set (ls -d style).
  NOTE: Only expands one level of a directory.
`
}

func (p *lsCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.long, "long", false, "If true prints out as ls -l would have done")
	f.BoolVar(&p.directory, "directory", false, "If true prints out the entry if it was a directory and nothing else (as ls -d)")
}

func (p *lsCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Please specify a filename to read.")
		return subcommands.ExitUsageError
	}

	retCode := subcommands.ExitSuccess
	for _, filename := range f.Args() {
		req := &pb.ListRequest{
			Entry: filename,
		}

		c := pb.NewLocalFileClientProxy(state.Conn)
		stream, err := c.ListOneMany(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error from ListOneMany for %s: %v\n", filename, err)
			return subcommands.ExitFailure
		}

		type seen struct {
			seen bool
			skip bool
		}
		// Track the responses per target (need to do special work on the first one)
		entries := make(map[int]*seen)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error from ListOneMany.Recv() for %s %v\n", filename, err)
				retCode = subcommands.ExitFailure
				break
			}

			for _, r := range resp {
				if r.Error != nil {
					if r.Error != io.EOF {
						fmt.Fprintf(state.Out[r.Index], "Got error from target %s (%d) - %v\n", r.Target, r.Index, r.Error)
						retCode = subcommands.ExitFailure
					}
					continue
				}

				// First entry is special. It may be a directory if that's what we asked to List on.
				if entries[r.Index] == nil {
					entries[r.Index] = &seen{
						seen: true,
					}
					if fs.FileMode(r.Resp.Entry.Mode).IsDir() {
						// If -d is set we print just this and then skip anymore for this target.
						// Otherwise we skip this entry and keep going.
						if p.directory {
							entries[r.Index].skip = true
						} else {
							continue
						}
					}
				} else {
					if entries[r.Index].skip {
						continue
					}
				}

				// Easy case. Just emit entries
				if !p.long {
					fmt.Fprintf(state.Out[r.Index], "%s\n", r.Resp.Entry.Filename)
					continue
				}
				t := fs.FileMode(r.Resp.Entry.Mode).String()
				mod := r.Resp.Entry.Modtime.AsTime().Format(time.UnixDate)
				fmt.Fprintf(state.Out[r.Index], "%-11s  - %8d %8d %16d %s %s\n", t, r.Resp.Entry.Uid, r.Resp.Entry.Gid, r.Resp.Entry.Size, mod, r.Resp.Entry.Filename)
			}
		}
	}
	return retCode
}
