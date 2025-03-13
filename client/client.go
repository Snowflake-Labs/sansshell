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

// Package client provides utility functions for gluing new commands
// easily into sanssh.
package client

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"os"
	"text/tabwriter"

	"github.com/google/subcommands"
)

// HasSubpackage should be implemented by users of SetupSubpackage to
// help with introspecting subpackages of subcommands.
type HasSubpackage interface {
	GetSubpackage(f *flag.FlagSet) *subcommands.Commander
}

// SetupSubpackage is a helper to create a Commander to hold the actual
// commands run inside of a top-level command. The returned Commander should
// then have the relevant sub-commands registered within it.
func SetupSubpackage(name string, f *flag.FlagSet) *subcommands.Commander {
	c := subcommands.NewCommander(f, name)
	c.Register(c.HelpCommand(), "")
	c.Register(c.FlagsCommand(), "")
	c.Register(c.CommandsCommand(), "")
	return c
}

// GenerateSynopsis will generate a consistent snnopysis for a top level command
// with N sub-commands contained within it. The leading param indicates the number
// of leading tabs to generate before the name and synopsis. Generally this is 2
// unless you're a subcommand of a subcommand and then you'll want more.
func GenerateSynopsis(c *subcommands.Commander, leading int) string {
	b := &bytes.Buffer{}
	w := tabwriter.NewWriter(b, 8, 0, 2, ' ', 0)
	if _, err := w.Write([]byte("\n")); err != nil {
		panic(fmt.Sprintf("buffer write failed: %v", err))
	}
	fn := func(_ *subcommands.CommandGroup, comm subcommands.Command) {
		switch comm.Name() {
		case "help", "flags", "commands":
			break
		default:
			for i := 0; i < leading; i++ {
				fmt.Fprintf(w, "\t")
			}
			fmt.Fprintf(w, "%s\t%s\t\n", comm.Name(), comm.Synopsis())
		}
	}
	c.VisitCommands(fn)
	w.Flush()
	return b.String()
}

// GenerateUsage will return a usage string to a top level command with N
// sub-commands contained within it.
func GenerateUsage(name string, synopsis string) string {
	return fmt.Sprintf("%s has several subcommands. Pick one to perform the action you wish:\n%s", name, synopsis)
}

// PredictArgs can optionally be implemented to help with command-line completion. It will typically be implemented
// on a type that already implements subcommands.Command.
type PredictArgs interface {
	// PredictArgs returns prediction options for a given prefix. The prefix is
	// the subcommand arguments that have been typed so far (possibly nothing)
	// and can be used as a hint for what to return. The returned values will be
	// automatically filtered by the prefix when needed.
	PredictArgs(prefix string) []string
}

// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// TODO: refactor before release
// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
type SansshellCommandController[TFlags interface{}, TReq interface{}, TResp interface{}] struct {
	Name             string
	Synopsis         string
	Usage            string
	Flags            func(f *flag.FlagSet) TFlags
	GetGRPCRequest   func(ctx context.Context, flags TFlags, state *util.ExecuteState, args ...interface{}) (TReq, error)
	SendGRPCRequest  func(ctx context.Context, state *util.ExecuteState, req TReq) (<-chan TResp, error)
	HandleSingleResp func(ctx context.Context, state *util.ExecuteState, resp TResp) subcommands.ExitStatus
}

type commandAdapter[TFlags interface{}, TReq interface{}, TResp interface{}] struct {
	ctl   *SansshellCommandController[TFlags, TReq, TResp]
	flags TFlags
}

func (a *commandAdapter[TFlags, TReq, TResp]) Name() string     { return a.ctl.Name }
func (a *commandAdapter[TFlags, TReq, TResp]) Synopsis() string { return a.ctl.Synopsis }
func (a *commandAdapter[TFlags, TReq, TResp]) Usage() string    { return a.ctl.Usage }
func (a *commandAdapter[TFlags, TReq, TResp]) SetFlags(f *flag.FlagSet) {
	a.flags = a.ctl.Flags(f)
}
func (a *commandAdapter[TFlags, TReq, TResp]) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if state == nil {
		panic("state is nil")
	}

	req, err := a.ctl.GetGRPCRequest(ctx, a.flags, args[0].(*util.ExecuteState), args...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return subcommands.ExitFailure
	}

	if state.AuthzDryRun {
		// TODO: send authz dry run request
		fmt.Fprintln(os.Stderr, "authz-dry-run is not supported for this command")
		return subcommands.ExitFailure
	}

	resp, err := a.ctl.SendGRPCRequest(ctx, state, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "All targets - Run returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	exitCode := subcommands.ExitSuccess
	for singleResp := range resp {
		respExistCode := a.ctl.HandleSingleResp(ctx, state, singleResp)
		if respExistCode != subcommands.ExitSuccess {
			exitCode = respExistCode
		}
	}

	return exitCode
}

func NewCommandAdapter[TFlags interface{}, TReq interface{}, TResp interface{}](ctl *SansshellCommandController[TFlags, TReq, TResp]) subcommands.Command {
	return &commandAdapter[TFlags, TReq, TResp]{ctl: ctl}
}
