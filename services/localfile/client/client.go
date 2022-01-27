/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

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

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "file"

func init() {
	subcommands.Register(&fileCmd{}, subPackage)
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&chgrpCmd{}, "")
	c.Register(&chmodCmd{}, "")
	c.Register(&chownCmd{}, "")
	c.Register(&cpCmd{}, "")
	c.Register(&immutableCmd{}, "")
	c.Register(&lsCmd{}, "")
	c.Register(&readCmd{}, "")
	c.Register(&statCmd{}, "")
	c.Register(&sumCmd{}, "")
	c.Register(&tailCmd{}, "")
	return c
}

type fileCmd struct{}

func (*fileCmd) Name() string { return subPackage }
func (p *fileCmd) Synopsis() string {
	return client.GenerateSynopsis(setup(flag.NewFlagSet("", flag.ContinueOnError)))
}
func (p *fileCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*fileCmd) SetFlags(f *flag.FlagSet) {}

func (p *fileCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setup(f)
	return c.Execute(ctx, args...)
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

	c := pb.NewLocalFileClientProxy(state.Conn)

	retCode := subcommands.ExitSuccess
	for _, filename := range f.Args() {
		req := &pb.ListRequest{
			Entry: filename,
		}

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

type cpCmd struct {
	bucket    string
	overwrite bool
	uid       int
	gid       int
	mode      int
	immutable bool
}

func (*cpCmd) Name() string     { return "cp" }
func (*cpCmd) Synopsis() string { return "Copy a file onto a remote machine." }
func (*cpCmd) Usage() string {
	return `cp [--bucket=XXX] [--overwrite] --uid=X --gid=X --mode=X [--immutable] <source> <remote destination>
  Copy the source file (which can be local or a URL such as s3://bucket/source) to the target(s)
  placing it into the remote destination.
`
}

func (p *cpCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.bucket, "bucket", "", "If set to a valid prefix will copy from this bucket with the key being the source provided")
	f.BoolVar(&p.overwrite, "overwrite", false, "If true will overwrite the remote file. Otherwise the file pre-existing is an error.")
	f.IntVar(&p.uid, "uid", -1, "The uid the remote file will be set via chown.")
	f.IntVar(&p.gid, "gid", -1, "The gid the remote file will be set via chown.")
	f.IntVar(&p.mode, "mode", -1, "The mode the remote file will be set via chmod.")
	f.BoolVar(&p.immutable, "immutable", false, "If true sets the remote file to immutable after being written.")
}

var validOutputPrefixes = []string{
	"s3://",
	"azblob://",
	"gs://",
}

func (p *cpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Please specify a source to copy and destination filename to write the contents into.")
		return subcommands.ExitUsageError
	}
	if p.uid == -1 || p.gid == -1 || p.mode == -1 {
		fmt.Fprintln(os.Stderr, "Must set --uid, --gid and --mode")
		return subcommands.ExitUsageError
	}

	c := pb.NewLocalFileClientProxy(state.Conn)
	source := f.Args()[0]
	dest := f.Args()[1]

	if p.bucket != "" {
		valid := false
		for _, pre := range validOutputPrefixes {
			if strings.HasPrefix(p.bucket, pre) {
				valid = true
				break
			}
		}
		if !valid {
			fmt.Fprintf(os.Stderr, "Invalid bucket %s. Valid ones accepted are:\n\n", p.bucket)
			for _, p := range validOutputPrefixes {
				fmt.Fprintf(os.Stderr, "%s\n", p)
			}
			return subcommands.ExitUsageError
		}
	}

	descr := &pb.FileWrite{
		Attrs: &pb.FileAttributes{
			Filename: dest,
			Attributes: []*pb.FileAttribute{
				{
					Value: &pb.FileAttribute_Uid{
						Uid: uint32(p.uid),
					},
				},
				{
					Value: &pb.FileAttribute_Gid{
						Gid: uint32(p.gid),
					},
				},
				{
					Value: &pb.FileAttribute_Mode{
						Mode: uint32(p.mode),
					},
				},
				{
					Value: &pb.FileAttribute_Immutable{
						Immutable: p.immutable,
					},
				},
			},
		},
		Overwrite: p.overwrite,
	}

	// Copy case (simpler)
	if p.bucket != "" {
		req := &pb.CopyRequest{
			Destination: descr,
			Bucket:      p.bucket,
			Key:         source,
		}

		respChan, err := c.CopyOneMany(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error from CopyOneMany: %v\n", err)
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

	// Write case (have to send over the local file).
	f1, err := os.Open(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't open %s - %v\n", source, err)
		return subcommands.ExitFailure
	}
	defer f1.Close()

	stream, err := c.WriteOneMany(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error from WriteOneMany: %v\n", err)
		return subcommands.ExitFailure
	}

	// Send over the description and then we'll loop sending contents.
	req := &pb.WriteRequest{
		Request: &pb.WriteRequest_Description{
			Description: descr,
		},
	}
	if err := stream.Send(req); err != nil {
		fmt.Fprintf(os.Stderr, "Error sending on stream - %v\n", err)
		return subcommands.ExitFailure
	}

	buf := make([]byte, util.StreamingChunkSize)
	for {
		n, err := f1.Read(buf)
		if err == io.EOF {
			break
		}

		req := &pb.WriteRequest{
			Request: &pb.WriteRequest_Contents{
				// Only send up to n as the last read is often a short read.
				Contents: buf[:n],
			},
		}
		if err := stream.Send(req); err != nil {
			fmt.Fprintf(os.Stderr, "Error sending on stream - %v\n", err)
			return subcommands.ExitFailure
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "Error closing stream - %v\n", err)
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess

	// There are no responses to process but we do need to check for errors.
	for _, r := range resp {
		if r.Error != nil && r.Error != io.EOF {
			fmt.Fprintf(state.Out[r.Index], "Got error from target %s (%d) - %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}
