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
	"flag"
	"fmt"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
	"io"
	"io/fs"
	"os"
	"slices"
)

type rmdirCmd struct {
	recursive bool
}

func (*rmdirCmd) Name() string     { return "rmdir" }
func (*rmdirCmd) Synopsis() string { return "Remove a directory." }
func (*rmdirCmd) Usage() string {
	return `rm [-r] <directory>:
  Remove the given directory.
  `
}

func (i *rmdirCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&i.recursive, "r", false, "Attempt to delete all files and sub-dirs before deleting specified dir")
}

func (i *rmdirCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "please specify a directory to rm")
		return subcommands.ExitUsageError
	}

	if i.recursive {
		return RmdirRecursive(ctx, state, f, args)
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

type stringSet map[string]struct{}

func (ss stringSet) Merge(other stringSet) {
	for k := range other {
		ss[k] = struct{}{}
	}
}

type fileTreeInfo struct {
	Dirs  stringSet
	Files stringSet
}

func (le *fileTreeInfo) Merge(other *fileTreeInfo) {
	le.Dirs.Merge(other.Dirs)
	le.Files.Merge(other.Files)
}

func RmdirRecursive(ctx context.Context, state *util.ExecuteState, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	client := pb.NewLocalFileClientProxy(state.Conn)
	rootPath := f.Arg(0)

	rootFTInfo, err := listDirsAndFiles(ctx, client, rootPath)
	if err != nil {
		return subcommands.ExitFailure
	}

	dirsToExplore := rootFTInfo.Dirs
	for len(dirsToExplore) > 0 {
		newDirsToExplore := stringSet{}
		for dir := range dirsToExplore {
			newFTInfo, err := listDirsAndFiles(ctx, client, dir)
			if err != nil {
				continue
			}
			newDirsToExplore.Merge(newFTInfo.Dirs)
			rootFTInfo.Merge(newFTInfo)
		}
		dirsToExplore = newDirsToExplore
	}

	dirs := make([]string, 0, len(rootFTInfo.Dirs))
	files := make([]string, 0, len(rootFTInfo.Files))

	for dir := range rootFTInfo.Dirs {
		dirs = append(dirs, dir)
	}
	for file := range rootFTInfo.Files {
		files = append(files, file)
	}

	slices.Sort(dirs)
	dirs = append([]string{rootPath}, dirs...)

	for _, file := range files {
		respCh, err := client.RmOneMany(ctx, &pb.RmRequest{
			Filename: file,
		})
		if err != nil {
			fmt.Printf("Failed to delete file: %s\n", file)
			continue
		}
		for r := range respCh {
			if r.Error == nil {
				continue
			}
			fmt.Printf("Failed to delete file %s on target %s: %v\n", file, r.Target, r.Error)
		}
	}

	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		respCh, err := client.RmdirOneMany(ctx, &pb.RmdirRequest{
			Directory: dir,
		})
		if err != nil {
			fmt.Printf("Failed to delete dir: %s\n", dir)
			continue
		}
		for r := range respCh {
			if r.Error == nil {
				continue
			}
			fmt.Printf("Failed to delete dir %s on target %s: %v\n", dir, r.Target, r.Error)
		}
	}

	return subcommands.ExitSuccess
}

func listDirsAndFiles(ctx context.Context, client pb.LocalFileClientProxy, dir string) (*fileTreeInfo, error) {
	stream, err := client.ListOneMany(ctx, &pb.ListRequest{
		Entry: dir,
	})

	if err != nil {
		return nil, err
	}

	ftInfo := &fileTreeInfo{
		Dirs:  stringSet{},
		Files: stringSet{},
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("error during path collection: %s\n", err)
			break
		}

		for _, msg := range resp {
			if msg.Error == io.EOF {
				continue
			}
			if msg.Error != nil {
				fmt.Println(msg.Error)
				continue
			}

			statEntry := msg.Resp.GetEntry()
			isDir := fs.FileMode(statEntry.Mode).IsDir()
			if statEntry.Filename == dir {
				continue
			}
			if isDir {
				ftInfo.Dirs[statEntry.Filename] = struct{}{}
			} else {
				ftInfo.Files[statEntry.Filename] = struct{}{}
			}
		}
	}

	return ftInfo, nil
}
