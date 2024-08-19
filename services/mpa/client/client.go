/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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

// Package client provides the client interface for 'mpa'
package client

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/mpa"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const subPackage = "mpa"

func init() {
	subcommands.Register(&mpaCmd{}, subPackage)
}

func (*mpaCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&approveCmd{}, "")
	c.Register(&listCmd{}, "")
	c.Register(&getCmd{}, "")
	c.Register(&clearCmd{}, "")
	return c
}

type mpaCmd struct{}

func (*mpaCmd) Name() string { return subPackage }
func (p *mpaCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *mpaCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*mpaCmd) SetFlags(f *flag.FlagSet) {}

func (p *mpaCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

func getAction(ctx context.Context, state *util.ExecuteState, c pb.MpaClientProxy, id string) *pb.Action {
	resp, err := c.GetOneMany(ctx, &pb.GetRequest{Id: id})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
		}
		return nil
	}
	var anyAction *pb.Action
	actions := make(map[int]*pb.Action)
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Unable to look up request: %v\n", r.Error)
			continue
		}
		if r.Resp.Action == nil {
			fmt.Fprintf(state.Err[r.Index], "Error: action was nil when looking up MPA request")
			continue
		}
		actions[r.Index] = r.Resp.Action
		anyAction = r.Resp.Action
	}
	if anyAction == nil {
		// All commands above must have failed for this to happen.
		return nil
	}
	for _, a := range actions {
		if !proto.Equal(a, anyAction) {
			// Bail if returned actions were inconsistent because we don't know which action is
			// correct.
			for idx := range actions {
				fmt.Fprintf(state.Err[idx], "All targets - inconsistent action: <%v> vs <%v>\n", a, anyAction)
			}
			return nil
		}
	}
	return anyAction
}

type approveCmd struct {
	skipConfirmation bool
}

func (*approveCmd) Name() string     { return "approve" }
func (*approveCmd) Synopsis() string { return "Approves an MPA request" }
func (*approveCmd) Usage() string {
	return `approve <id> [--skip-confirmation]:
    Approves an MPA request with the specified ID.

	The --skip-confirmation flag can be used to bypass
	the confirmation prompt, proceeding with the request approval.
`
}

func (p *approveCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.skipConfirmation, "skip-confirmation", false, "If true won't ask for confirmation")
}

func (p *approveCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Please specify a single ID to approve.")
		return subcommands.ExitUsageError
	}
	id := f.Args()[0]
	c := pb.NewMpaClientProxy(state.Conn)
	action := getAction(ctx, state, c, id)
	if action == nil {
		return subcommands.ExitFailure
	}

	fmt.Printf("MPA Request:\n%s\n", protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Format(action))

	if !p.skipConfirmation {
		// ask for confirmation
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Printf("Would you like to approve the request? (yes/no): ")
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Error reading input. Please try again.")
				continue
			}
			input = strings.TrimSpace(input)
			if strings.ToLower(input) == "yes" || strings.ToLower(input) == "y" {
				break
			}
			fmt.Print("Request is not approved. Exiting.\n")
			return subcommands.ExitSuccess
		}
	}

	approved, err := c.ApproveOneMany(ctx, &pb.ApproveRequest{
		Action: action,
	})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
		}
		return subcommands.ExitFailure
	}
	for r := range approved {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Unable to approve: %v\n", r.Error)
			continue
		}
		msg := []string{"Approved", action.Method}
		if action.GetUser() != "" {
			msg = append(msg, "from", action.GetUser())
		}
		if action.GetJustification() != "" {
			msg = append(msg, "for", action.GetJustification())
		}
		fmt.Fprintln(state.Out[r.Index], strings.Join(msg, " "))
	}
	return subcommands.ExitSuccess
}

type listCmd struct {
	verbose bool
}

func (*listCmd) Name() string     { return "list" }
func (*listCmd) Synopsis() string { return "Lists out pending MPA requests on machines" }
func (*listCmd) Usage() string {
	return `list:
    Lists out any MPA requests on machines.
`
}

func (p *listCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.verbose, "v", false, "Verbose: list full details of MPA request")
}

func (p *listCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "List takes no args.")
		return subcommands.ExitUsageError
	}

	c := pb.NewMpaClientProxy(state.Conn)

	resp, err := c.ListOneMany(ctx, &pb.ListRequest{})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
		}
		return subcommands.ExitFailure
	}
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintln(state.Err[r.Index], r.Error)
			continue
		}
		for _, item := range r.Resp.Item {
			msg := []string{item.Id}
			if p.verbose {
				if len(item.Approver) > 0 {
					var approvers []string
					for _, a := range item.Approver {
						approvers = append(approvers, a.Id)
					}
					msg = append(msg, fmt.Sprintf("(approved by %v)", strings.Join(approvers, ",")))
				}
				msg = append(msg, protojson.MarshalOptions{UseProtoNames: true}.Format(item.Action))
			} else {
				msg = append(msg, item.Action.GetMethod())
				if item.Action.GetUser() != "" {
					msg = append(msg, "from", item.Action.GetUser())
				}
				if item.Action.GetJustification() != "" {
					msg = append(msg, "for", item.Action.GetJustification())
				}
				if len(item.Approver) > 0 {
					msg = append(msg, "(approved)")
				}
			}
			fmt.Fprintln(state.Out[r.Index], strings.Join(msg, " "))
		}
	}
	return subcommands.ExitSuccess
}

type clearCmd struct{}

func (*clearCmd) Name() string     { return "clear" }
func (*clearCmd) Synopsis() string { return "Clears an MPA request" }
func (*clearCmd) Usage() string {
	return `clear <id>:
    Clears an MPA request with the specified ID.
`
}

func (p *clearCmd) SetFlags(f *flag.FlagSet) {}

func (p *clearCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Please specify a single ID to clear.")
		return subcommands.ExitUsageError
	}
	c := pb.NewMpaClientProxy(state.Conn)
	action := getAction(ctx, state, c, f.Args()[0])
	if action == nil {
		return subcommands.ExitFailure
	}

	cleared, err := c.ClearOneMany(ctx, &pb.ClearRequest{
		Action: action,
	})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
		}
		return subcommands.ExitFailure
	}
	for r := range cleared {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Unable to clear: %v\n", r.Error)
			continue
		}
		fmt.Fprintln(state.Out[r.Index], "Cleared")
	}
	return subcommands.ExitSuccess
}

type getCmd struct{}

func (*getCmd) Name() string     { return "get" }
func (*getCmd) Synopsis() string { return "Print an MPA request" }
func (*getCmd) Usage() string {
	return `get <id>:
    Prints out the MPA request with the specified ID.
`
}

func (p *getCmd) SetFlags(f *flag.FlagSet) {}

func (p *getCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Please specify a single ID to approve.")
		return subcommands.ExitUsageError
	}
	c := pb.NewMpaClientProxy(state.Conn)
	resp, err := c.GetOneMany(ctx, &pb.GetRequest{Id: f.Args()[0]})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
		}
		return subcommands.ExitFailure
	}
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Unable to look up request: %v\n", r.Error)
			continue
		}
		if r.Resp.Action == nil {
			fmt.Fprintf(state.Err[r.Index], "Error: action was nil when looking up MPA request")
			continue
		}
		fmt.Fprintln(state.Out[r.Index], protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Format(r.Resp))
	}
	return subcommands.ExitSuccess
}
