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

// Package client provides the client interface for 'exec'
package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/exec"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

// Default value for using streaming exec.
// This is exposed so that clients can set a default that works for
// their environment. StreamingExec was added after Exec, so policies
// or sansshell nodes may not have been updated to support streaming.
var DefaultStreaming = false

const subPackage = "exec"

func init() {
	subcommands.Register(&execCmd{}, subPackage)
}

func (*execCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&runCmd{}, "")
	return c
}

type execCmd struct{}

func (*execCmd) Name() string { return subPackage }
func (p *execCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *execCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*execCmd) SetFlags(f *flag.FlagSet) {}

func (p *execCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

type runCmd struct {
	streaming bool

	// returnCode internally keeps track of the final status to return
	returnCode subcommands.ExitStatus
}

func (*runCmd) Name() string     { return "run" }
func (*runCmd) Synopsis() string { return "Run provided command and return a response." }
func (*runCmd) Usage() string {
	return `run <command> [--stream] [<args>...]:
  Run a command remotely and return the response

	Note: This is not optimized for large output or long running commands.  If
	the output doesn't fit in memory in a single proto message or if it doesn't
	complete within the timeout, you'll have a bad time.

	The --stream flag can be used to stream back command output as the command
	runs. It doesn't affect the timeout.
`
}

func (p *runCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.streaming, "stream", DefaultStreaming, "If true, stream back stdout and stdin during the command instead of sending it all at the end.")
}

func (p *runCmd) printCommandOutput(state *util.ExecuteState, idx int, resp *pb.ExecResponse, err error) {
	if err == io.EOF {
		// Streaming commands may return EOF
		return
	}
	if err != nil {
		fmt.Fprintf(state.Err[idx], "Command execution failure - %v\n", err)
		// If any target had errors it needs to be reported for that target but we still
		// need to process responses off the channel. Final return code though should
		// indicate something failed.
		p.returnCode = subcommands.ExitFailure
		return
	}
	if len(resp.Stderr) > 0 {
		fmt.Fprintf(state.Err[idx], "%s", resp.Stderr)
	}
	fmt.Fprintf(state.Out[idx], "%s", resp.Stdout)
	if resp.RetCode != 0 {
		p.returnCode = subcommands.ExitFailure
	}
}

func (p *runCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Please specify a command to execute.\n")
		return subcommands.ExitUsageError
	}

	c := pb.NewExecClientProxy(state.Conn)
	req := &pb.ExecRequest{Command: f.Args()[0], Args: f.Args()[1:]}
	if p.streaming {
		resp, err := c.StreamingRunOneMany(ctx, req)
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			for _, e := range state.Err {
				fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
			}
			return subcommands.ExitFailure
		}
		for {
			rs, err := resp.Recv()
			if err != nil {
				if err == io.EOF {
					return p.returnCode
				}
				fmt.Fprintf(os.Stderr, "Stream failure: %v\n", err)
				return subcommands.ExitFailure
			}
			for _, r := range rs {
				p.printCommandOutput(state, r.Index, r.Resp, r.Error)
			}
		}
	} else {
		resp, err := c.RunOneMany(ctx, req)
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			for _, e := range state.Err {
				fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
			}
			return subcommands.ExitFailure
		}

		for r := range resp {
			p.printCommandOutput(state, r.Index, r.Resp, r.Error)
		}
	}
	return p.returnCode
}
