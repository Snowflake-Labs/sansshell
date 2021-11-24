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

	"github.com/google/subcommands"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

func init() {
	subcommands.Register(&readCmd{}, "file")
	subcommands.Register(&tailCmd{}, "tail")
	subcommands.Register(&statCmd{}, "file")
	subcommands.Register(&sumCmd{}, "file")
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
		fmt.Fprintf(os.Stderr, "Please specify a filename to read.\n")
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
		fmt.Fprintf(os.Stderr, "Please specify a filename to tail.\n")
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

func (s *statCmd) Name() string     { return "stat" }
func (s *statCmd) Synopsis() string { return "Stat file(s) to stdout." }
func (s *statCmd) Usage() string {
	return `stat <path> [<path>...]:
  Stat one or more remote fles and print the result to stdout.
  `
}

func (s *statCmd) SetFlags(f *flag.FlagSet) {}

func (s *statCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)

	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "please specify at least one path to stat\n")
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

func (s *sumCmd) Name() string     { return "sum" }
func (s *sumCmd) Synopsis() string { return "Calculate file sums" }
func (s *sumCmd) Usage() string {
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
		fmt.Fprintf(os.Stderr, "please specify a filename to sum\n")
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
