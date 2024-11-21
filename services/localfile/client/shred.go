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
	"os"
)

type shredCmd struct {
	force  bool
	zero   bool
	remove bool
}

func (*shredCmd) Name() string     { return "shred" }
func (*shredCmd) Synopsis() string { return "Shred a file" }
func (*shredCmd) Usage() string {
	return `shred <path-to-file>`
}

func (p *shredCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.force, "f", false, "force permissions change in case current permissions are insufficient")
	f.BoolVar(&p.remove, "u", false, "remove file after shredding - same as --remove")
	f.BoolVar(&p.zero, "z", false, "add a zeroing pass after shredding to mask the shredding")
	f.BoolVar(&p.remove, "remove", false, "remove file after shredding - same as -u")
}

func (p *shredCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "please specify a filename to shred")
		return subcommands.ExitUsageError
	}

	req := &pb.ShredRequest{
		Filename: f.Arg(0),
		Force:    p.force,
		Zero:     p.zero,
		Remove:   p.remove,
	}

	client := pb.NewLocalFileClientProxy(state.Conn)
	respChan, err := client.ShredOneMany(ctx, req)

	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - shred error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range respChan {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "shred error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}
