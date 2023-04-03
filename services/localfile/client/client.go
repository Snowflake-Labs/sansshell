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

// Package client provides the client interface for 'file'
package client

import (
	"context"
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

func (*fileCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&chgrpCmd{}, "")
	c.Register(&chmodCmd{}, "")
	c.Register(&chownCmd{}, "")
	c.Register(&cpCmd{}, "")
	c.Register(&immutableCmd{}, "")
	c.Register(&lsCmd{}, "")
	c.Register(&readCmd{}, "")
	c.Register(&readlinkCmd{}, "")
	c.Register(&renameCmd{}, "")
	c.Register(&rmCmd{}, "")
	c.Register(&rmdirCmd{}, "")
	c.Register(&statCmd{}, "")
	c.Register(&symlinkCmd{}, "")
	c.Register(&sumCmd{}, "")
	c.Register(&tailCmd{}, "")
	return c
}

type fileCmd struct{}

func (*fileCmd) Name() string { return subPackage }
func (p *fileCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *fileCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*fileCmd) SetFlags(f *flag.FlagSet) {}

func (p *fileCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
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

	return readFile(ctx, state, req)
}

func readFile(ctx context.Context, state *util.ExecuteState, req *pb.ReadActionRequest) subcommands.ExitStatus {
	c := pb.NewLocalFileClientProxy(state.Conn)
	stream, err := c.ReadOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			filename := req.GetFile().Filename
			fmt.Fprintf(e, "All targets - could not read file %s: %v\n", filename, err)
		}
		return subcommands.ExitFailure
	}

	targetsDone := make(map[int]bool)
	exit := subcommands.ExitSuccess
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			// But...we only do this for targets that aren't complete. A complete target
			// didn't have an error. i.e. we got N done then the context expired.
			for i, e := range state.Err {
				if !targetsDone[i] {
					fmt.Fprintf(e, "Stream error: %v\n", err)
				}
			}
			exit = subcommands.ExitFailure
			break
		}
		for _, r := range resp {
			contents := r.Resp.Contents
			if r.Error != nil && r.Error != io.EOF {
				fmt.Fprintf(state.Err[r.Index], "Target %s (%d) returned error - %v\n", r.Target, r.Index, r.Error)
				targetsDone[r.Index] = true
				// If any target had errors it needs to be reported for that target but we still
				// need to process responses off the channel. Final return code though should
				// indicate something failed.
				exit = subcommands.ExitFailure
				continue
			}

			// At EOF this target is done.
			if r.Error == io.EOF {
				targetsDone[r.Index] = true
				continue
			}

			// If we haven't previously had a problem keep writing. Otherwise we drop this and just keep processing.
			if !targetsDone[r.Index] {
				n, err := state.Out[r.Index].Write(contents)
				if got, want := n, len(contents); got != want {
					fmt.Fprintf(state.Err[r.Index], "can't write into buffer at correct length. Got %d want %d\n", got, want)
					targetsDone[r.Index] = true
					exit = subcommands.ExitFailure
				}
				if err != nil {
					fmt.Fprintf(state.Err[r.Index], "error writing output to output index %d - %v\n", r.Index, err)
					targetsDone[r.Index] = true
					exit = subcommands.ExitFailure
				}
			}

		}
	}
	return exit
}

type readlinkCmd struct {
}

func (*readlinkCmd) Name() string { return "readlink" }
func (*readlinkCmd) Synopsis() string {
	return "Print the value of a symbolic link or canonical file name."
}
func (*readlinkCmd) Usage() string {
	return `readlink <path>
  Print the value of a symbolic link or canonical file named by <path>.
`
}

func (*readlinkCmd) SetFlags(f *flag.FlagSet) {}

func (p *readlinkCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Please specify a filename to read.")
		return subcommands.ExitUsageError
	}

	filename := f.Args()[0]
	req := &pb.ReadlinkRequest{
		Filename: filename,
	}

	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.ReadlinkOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - readlink client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "readlink client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		} else {
			fmt.Fprintln(state.Out[r.Index], r.Resp.Linkvalue)
		}
	}
	return retCode
}

type symlinkCmd struct {
}

func (*symlinkCmd) Name() string { return "readlink" }
func (*symlinkCmd) Synopsis() string {
	return "Create a symbolic link."
}
func (*symlinkCmd) Usage() string {
	return `symlink <target> <linkname>
  Create a symbolic link from target to linkname, similar to "ln -s <target> <linkname>".
`
}

func (*symlinkCmd) SetFlags(f *flag.FlagSet) {}

func (p *symlinkCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Please specify target and linkname.")
		return subcommands.ExitUsageError
	}

	target := f.Args()[0]
	linkname := f.Args()[1]
	req := &pb.SymlinkRequest{
		Target:   target,
		Linkname: linkname,
	}

	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.SymlinkOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - symlink client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "symlink client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
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

	return readFile(ctx, state, req)
}

type statCmd struct {
	followLinks bool
}

func (*statCmd) Name() string     { return "stat" }
func (*statCmd) Synopsis() string { return "Stat file(s) to stdout." }
func (*statCmd) Usage() string {
	return `stat <path> [<path>...]:
  Stat one or more remote fles and print the result to stdout.
  `
}

func (s *statCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&s.followLinks, "L", false, "Use stat instead of lstat (as stat -L)")
	f.BoolVar(&s.followLinks, "follow-links", false, "Use stat instead of lstat (as stat -L)")
}

func (s *statCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)

	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify at least one path to stat")
		return subcommands.ExitUsageError
	}
	client := pb.NewLocalFileClientProxy(state.Conn)
	stream, err := client.StatOneMany(ctx)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - stat client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	waitc := make(chan subcommands.ExitStatus)

	// Push all the sends into their own routine so we're receiving replies
	// at the same time. This way we can't deadlock if we send so much this
	// blocks which makes the server block its sends waiting on us to receive.
	go func() {
		e := subcommands.ExitSuccess
		for _, filename := range f.Args() {
			if err := stream.Send(&pb.StatRequest{Filename: filename, FollowLinks: s.followLinks}); err != nil {
				// Emit this to every error file as it's not specific to a given target.
				for _, e := range state.Err {
					fmt.Fprintf(e, "All targets - stat: send error: %v\n", err)
				}
				e = subcommands.ExitFailure
				break
			}
		}
		// Close the sending stream to notify the server not to expect any further data.
		// We'll process below but this let's the server politely know we're done sending
		// as otherwise it'll see this as a cancellation.
		if err := stream.CloseSend(); err != nil {
			// Emit this to every error file as it's not specific to a given target.
			for _, e := range state.Err {
				fmt.Fprintf(e, "All targets - stat: CloseSend error: %v\n", err)
			}
			e = subcommands.ExitFailure
		}

		waitc <- e
		close(waitc)
	}()

	targetsDone := make(map[int]bool)
	retCode := subcommands.ExitSuccess
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			// But...we only do this for targets that aren't complete. A complete target
			// didn't have an error. i.e. we got N done then the context expired.
			for i, e := range state.Err {
				if !targetsDone[i] {
					fmt.Fprintf(e, "stat: receive error: %v\n", err)
				}
			}
			retCode = subcommands.ExitFailure
			break
		}
		for _, r := range reply {
			if r.Error != nil && r.Error != io.EOF {
				fmt.Fprintf(state.Err[r.Index], "Target %s (%d) returned error - %v\n", r.Target, r.Index, r.Error)
				targetsDone[r.Index] = true
				// If any target had errors it needs to be reported for that target but we still
				// need to process responses off the channel. Final return code though should
				// indicate something failed.
				retCode = subcommands.ExitFailure
				continue
			}

			// At EOF this target is done.
			if r.Error == io.EOF {
				targetsDone[r.Index] = true
				continue
			}

			mode := os.FileMode(r.Resp.Mode)
			outTmpl := "File: %s\nSize: %d\nType: %s\nAccess: %s Uid: %d Gid: %d\nModify: %s\nImmutable: %t\n"
			fmt.Fprintf(state.Out[r.Index], outTmpl, r.Resp.Filename, r.Resp.Size, fileTypeString(mode), mode, r.Resp.Uid, r.Resp.Gid, r.Resp.Modtime.AsTime(), r.Resp.Immutable)
		}
	}
	for e := range waitc {
		retCode = e
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
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - sum client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	waitc := make(chan subcommands.ExitStatus)

	// Push all the sends into their own routine so we're receiving replies
	// at the same time. This way we can't deadlock if we send so much this
	// blocks which makes the server block its sends waiting on us to receive.
	go func() {
		e := subcommands.ExitSuccess
		for _, filename := range f.Args() {
			if err := stream.Send(&pb.SumRequest{Filename: filename, SumType: sumType}); err != nil {
				// Emit this to every error file as it's not specific to a given target.
				for _, e := range state.Err {
					fmt.Fprintf(e, "All targets - sum: send error: %v\n", err)
				}
				e = subcommands.ExitFailure
				break
			}
		}
		// Close the sending stream to notify the server not to expect any further data.
		// We'll process below but this let's the server politely know we're done sending
		// as otherwise it'll see this as a cancellation.
		if err := stream.CloseSend(); err != nil {
			// Emit this to every error file as it's not specific to a given target.
			for _, e := range state.Err {
				fmt.Fprintf(e, "All targets - sum: CloseSend error: %v\n", err)
			}
			e = subcommands.ExitFailure
		}
		waitc <- e
		close(waitc)
	}()

	targetsDone := make(map[int]bool)
	retCode := subcommands.ExitSuccess
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			// But...we only do this for targets that aren't complete. A complete target
			// didn't have an error. i.e. we got N done then the context expired.
			for i, e := range state.Err {
				if !targetsDone[i] {
					fmt.Fprintf(e, "stat: receive error: %v\n", err)
				}
			}
			retCode = subcommands.ExitFailure
			break
		}
		for _, r := range reply {
			if r.Error != nil && r.Error != io.EOF {
				fmt.Fprintf(state.Err[r.Index], "Got error from target %s (%d) - %v\n", r.Target, r.Index, r.Error)
				targetsDone[r.Index] = true
				// If any target had errors it needs to be reported for that target but we still
				// need to process responses off the channel. Final return code though should
				// indicate something failed.
				retCode = subcommands.ExitFailure
				continue
			}

			// At EOF this target is done.
			if r.Error == io.EOF {
				targetsDone[r.Index] = true
				continue
			}

			fmt.Fprintf(state.Out[r.Index], "%s %s\n", r.Resp.Sum, r.Resp.Filename)
		}
	}

	for e := range waitc {
		retCode = e
	}
	return retCode
}

type chownCmd struct {
	uid      int
	username string
}

func (*chownCmd) Name() string     { return "chown" }
func (*chownCmd) Synopsis() string { return "Change ownership on file/directory" }
func (*chownCmd) Usage() string {
	return `chown --uid=X|username=Y <path>:
  Change ownership on a file/directory.
  `
}

func (c *chownCmd) SetFlags(f *flag.FlagSet) {
	f.IntVar(&c.uid, "uid", -1, "Sets the file/directory to be owned by this uid")
	f.StringVar(&c.username, "username", "", "Sets the file/directory to be owned by this username")
}

func (c *chownCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify a filename to chown")
		return subcommands.ExitUsageError
	}
	if c.uid < 0 && c.username == "" {
		fmt.Fprintln(os.Stderr, "one of --uid or --username must be set (uid must be a positive number if used)")
		return subcommands.ExitFailure
	}
	if c.uid >= 0 && c.username != "" {
		fmt.Fprintln(os.Stderr, "cannot set both --uid and --username")
		return subcommands.ExitFailure
	}
	req := &pb.SetFileAttributesRequest{
		Attrs: &pb.FileAttributes{
			Filename: f.Args()[0],
		},
	}
	if c.uid >= 0 {
		req.Attrs.Attributes = append(req.Attrs.Attributes, &pb.FileAttribute{
			Value: &pb.FileAttribute_Uid{
				Uid: uint32(c.uid),
			},
		})
	}
	if c.username != "" {
		req.Attrs.Attributes = append(req.Attrs.Attributes, &pb.FileAttribute{
			Value: &pb.FileAttribute_Username{
				Username: c.username,
			},
		})
	}

	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.SetFileAttributesOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - chown client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "chown client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

type chgrpCmd struct {
	gid   int
	group string
}

func (*chgrpCmd) Name() string     { return "chgrp" }
func (*chgrpCmd) Synopsis() string { return "Change group membership on a file/directory" }
func (*chgrpCmd) Usage() string {
	return `chgrp --gid=X|group==Y <path>:
  Change group membership on a file/directory.
  `
}

func (c *chgrpCmd) SetFlags(f *flag.FlagSet) {
	f.IntVar(&c.gid, "gid", -1, "Sets the file/directory group membership specified by this gid")
	f.StringVar(&c.group, "group", "", "Sets the file/directory group membership specified by this name")
}

func (c *chgrpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "please specify a filename to chgrp")
		return subcommands.ExitUsageError
	}
	if c.gid < 0 && c.group == "" {
		fmt.Fprintln(os.Stderr, "one of --gid or --group must be set (gid must be a positive number if used)")
		return subcommands.ExitFailure
	}
	if c.gid >= 0 && c.group != "" {
		fmt.Fprintln(os.Stderr, "cannot set both --gid and --group")
		return subcommands.ExitFailure
	}

	req := &pb.SetFileAttributesRequest{
		Attrs: &pb.FileAttributes{
			Filename: f.Args()[0],
		},
	}
	if c.gid >= 0 {
		req.Attrs.Attributes = append(req.Attrs.Attributes, &pb.FileAttribute{
			Value: &pb.FileAttribute_Gid{
				Gid: uint32(c.gid),
			},
		})
	}
	if c.group != "" {
		req.Attrs.Attributes = append(req.Attrs.Attributes, &pb.FileAttribute{
			Value: &pb.FileAttribute_Group{
				Group: c.group,
			},
		})
	}

	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.SetFileAttributesOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - chgrp client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "chgrp client error: %v\n", r.Error)
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
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - chmod client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "chmod client error: %v\n", r.Error)
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
		fmt.Fprintln(os.Stderr, "please specify a filename to change immutable state")
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
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - immutable client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "immutable client error: %v\n", r.Error)
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
	f.BoolVar(&p.long, "l", false, "If true prints out as ls -l would have done")
	f.BoolVar(&p.long, "long", false, "If true prints out as ls -l would have done")
	f.BoolVar(&p.directory, "d", false, "If true prints out the entry if it was a directory and nothing else (as ls -d)")
	f.BoolVar(&p.directory, "directory", false, "If true prints out the entry if it was a directory and nothing else (as ls -d)")
}

func (p *lsCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Please specify a filename to read.")
		return subcommands.ExitUsageError
	}

	c := pb.NewLocalFileClientProxy(state.Conn)

	targetsDone := make(map[int]bool)
	retCode := subcommands.ExitSuccess
	for _, filename := range f.Args() {
		req := &pb.ListRequest{
			Entry: filename,
		}

		stream, err := c.ListOneMany(ctx, req)
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			for _, e := range state.Err {
				fmt.Fprintf(e, "All targets - error for %s: %v\n", filename, err)
			}
			retCode = subcommands.ExitFailure
			continue
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
				// Emit this to every error file as it's not specific to a given target.
				// But...we only do this for targets that aren't complete. A complete target
				// didn't have an error. i.e. we got N done then the context expired.
				for i, e := range state.Err {
					if !targetsDone[i] {
						fmt.Fprintf(e, "Error from ListOneMany.Recv() for %s %v\n", filename, err)
					}
				}
				retCode = subcommands.ExitFailure
				break
			}

			for _, r := range resp {
				if r.Error != nil && r.Error != io.EOF {
					fmt.Fprintf(state.Err[r.Index], "Got error from target %s (%d) - %v\n", r.Target, r.Index, r.Error)
					targetsDone[r.Index] = true
					// If any target had errors it needs to be reported for that target but we still
					// need to process responses off the channel. Final return code though should
					// indicate something failed.
					retCode = subcommands.ExitFailure
					continue
				}

				// At EOF this target is done.
				if r.Error == io.EOF {
					targetsDone[r.Index] = true
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
	username  string
	gid       int
	group     string
	mode      int
	immutable bool
}

func (*cpCmd) Name() string     { return "cp" }
func (*cpCmd) Synopsis() string { return "Copy a file on/onto a remote machine." }
func (*cpCmd) Usage() string {
	return `cp [--bucket=XXX] [--overwrite] --uid=X|username=Y --gid=X|group=Y --mode=X [--immutable] <source> <remote destination>
  Copy the source file (which can be local or a URL such as --bucket=s3://bucket <source> or --bucket=file://directory <source>) to the target(s)
  placing it into the remote destination.

NOTE: Using file:// means the file must be in that location on each remote target in turn as no data is transferred in that case. Also make
sure to use a fully formed directory. i.e. copying /etc/hosts would be --bucket=file:///etc hosts <destination>
`
}

func (p *cpCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.bucket, "bucket", "", "If set to a valid prefix will copy from this bucket with the key being the source provided")
	f.BoolVar(&p.overwrite, "overwrite", false, "If true will overwrite the remote file. Otherwise the file pre-existing is an error.")
	f.IntVar(&p.uid, "uid", -1, "The uid the remote file will be set via chown.")
	f.IntVar(&p.gid, "gid", -1, "The gid the remote file will be set via chown.")
	f.IntVar(&p.mode, "mode", -1, "The mode the remote file will be set via chmod.")
	f.BoolVar(&p.immutable, "immutable", false, "If true sets the remote file to immutable after being written.")
	f.StringVar(&p.username, "username", "", "The remote file will be set to this username via chown.")
	f.StringVar(&p.group, "group", "", "The remote file will be set to this group via chown.")
}

var validOutputPrefixes = []string{
	"s3://",
	"azblob://",
	"gs://",
	"file://",
}

func (p *cpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Please specify a source to copy and destination filename to write the contents into.")
		return subcommands.ExitUsageError
	}
	if (p.uid == -1 && p.username == "") || (p.gid == -1 && p.group == "") || p.mode == -1 {
		fmt.Fprintln(os.Stderr, "Must set --uid|username, --gid|group and --mode")
		return subcommands.ExitUsageError
	}
	if p.uid >= 0 && p.username != "" {
		fmt.Fprintln(os.Stderr, "cannot set both --uid and --username")
		return subcommands.ExitFailure
	}
	if p.gid >= 0 && p.group != "" {
		fmt.Fprintln(os.Stderr, "cannot set both --gid and --group")
		return subcommands.ExitFailure
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
	if p.uid >= 0 {
		descr.Attrs.Attributes = append(descr.Attrs.Attributes, &pb.FileAttribute{
			Value: &pb.FileAttribute_Uid{
				Uid: uint32(p.uid),
			},
		})
	}
	if p.username != "" {
		descr.Attrs.Attributes = append(descr.Attrs.Attributes, &pb.FileAttribute{
			Value: &pb.FileAttribute_Username{
				Username: p.username,
			},
		})
	}
	if p.gid >= 0 {
		descr.Attrs.Attributes = append(descr.Attrs.Attributes, &pb.FileAttribute{
			Value: &pb.FileAttribute_Gid{
				Gid: uint32(p.gid),
			},
		})
	}
	if p.group != "" {
		descr.Attrs.Attributes = append(descr.Attrs.Attributes, &pb.FileAttribute{
			Value: &pb.FileAttribute_Group{
				Group: p.group,
			},
		})
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
			// Emit this to every error file as it's not specific to a given target.
			for _, e := range state.Err {
				fmt.Fprintf(e, "All targets - error copying: %v\n", err)
			}
			return subcommands.ExitFailure
		}
		retCode := subcommands.ExitSuccess
		for r := range respChan {
			if r.Error != nil {
				fmt.Fprintf(state.Err[r.Index], "Got error from target %s (%d) - %v\n", r.Target, r.Index, r.Error)
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
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - error copying: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	// Send over the description and then we'll loop sending contents.
	req := &pb.WriteRequest{
		Request: &pb.WriteRequest_Description{
			Description: descr,
		},
	}
	if err := stream.Send(req); err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - error sending on stream - %v\n", err)
		}
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
			// Emit this to every error file as it's not specific to a given target.
			for _, e := range state.Err {
				fmt.Fprintf(e, "All targets - error sending on stream - %v\n", err)
			}
			return subcommands.ExitFailure
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - error closing stream - %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess

	// There are no responses to process but we do need to check for errors.
	for _, r := range resp {
		if r.Error != nil && r.Error != io.EOF {
			fmt.Fprintf(state.Err[r.Index], "Got error from target %s (%d) - %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

type rmCmd struct {
}

func (*rmCmd) Name() string     { return "rm" }
func (*rmCmd) Synopsis() string { return "Remove a file." }
func (*rmCmd) Usage() string {
	return `rm <filename>:
  Remove the given filename.
  `
}

func (i *rmCmd) SetFlags(f *flag.FlagSet) {}

func (i *rmCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "please specify a filename to rm")
		return subcommands.ExitUsageError
	}

	req := &pb.RmRequest{
		Filename: f.Arg(0),
	}
	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.RmOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - rm client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "rm client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

type rmdirCmd struct {
}

func (*rmdirCmd) Name() string     { return "rmdir" }
func (*rmdirCmd) Synopsis() string { return "Remove a directory." }
func (*rmdirCmd) Usage() string {
	return `rm <directory>:
  Remove the given directory.
  `
}

func (i *rmdirCmd) SetFlags(f *flag.FlagSet) {}

func (i *rmdirCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "please specify a directory to rm")
		return subcommands.ExitUsageError
	}

	req := &pb.RmdirRequest{
		Directory: f.Arg(0),
	}
	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.RmdirOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - rmdir client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "rm client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

type renameCmd struct {
}

func (*renameCmd) Name() string     { return "mv" }
func (*renameCmd) Synopsis() string { return "Rename a file/directory." }
func (*renameCmd) Usage() string {
	return `mv <old> <new>:
  Rename the given file/directory.
  `
}

func (i *renameCmd) SetFlags(f *flag.FlagSet) {}

func (i *renameCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "please specify the old and new names")
		return subcommands.ExitUsageError
	}

	req := &pb.RenameRequest{
		OriginalName:    f.Arg(0),
		DestinationName: f.Arg(1),
	}
	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.RenameOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - rmdir client error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "mv client error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}
