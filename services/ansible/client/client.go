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

// Package client provides the client interface for 'ansible'
package client

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/ansible"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "ansible"

func init() {
	subcommands.Register(&ansibleCmd{}, subPackage)
}

func (*ansibleCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(NewPlaybookController(), "")
	return c
}

type ansibleCmd struct{}

func (*ansibleCmd) Name() string { return subPackage }
func (p *ansibleCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *ansibleCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*ansibleCmd) SetFlags(f *flag.FlagSet) {}

func (p *ansibleCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

type playbookFlags struct {
	playbook string
	vars     util.KeyValueSliceFlag
	user     string
	check    bool
	diff     bool
	verbose  bool
}

func NewPlaybookController() subcommands.Command {
	controller := client.SansshellCommandController[*playbookFlags, *pb.RunRequest, *pb.RunManyResponse]{
		Name:     "playbook",
		Synopsis: "Run an ansible playbook on the server.",
		Usage:    `ansible: Run an ansible playbook on the remote server.`,
		Flags: func(f *flag.FlagSet) *playbookFlags {
			flags := &playbookFlags{}
			f.StringVar(&flags.playbook, "playbook", "", "The absolute path to the playbook to execute on the remote server.")
			f.Var(&flags.vars, "vars", "Pass key=value (via -e) to ansible-playbook. Multiple values can be specified separated by commas")
			f.StringVar(&flags.user, "user", "", "Run the playbook as this user")
			f.BoolVar(&flags.check, "check", false, "If true the playbook will be run with --check passed as an argument")
			f.BoolVar(&flags.diff, "diff", false, "If true the playbook will be run with --diff passed as an argument")
			f.BoolVar(&flags.verbose, "verbose", false, "If true the playbook wiill be run with -vvv passed as an argument")
			return flags
		},
		GetGRPCRequest: func(ctx context.Context, flags *playbookFlags, state *util.ExecuteState, args ...interface{}) (*pb.RunRequest, error) {
			if flags.playbook == "" {
				return nil, fmt.Errorf("--playbook is required")
			}

			req := &pb.RunRequest{
				Playbook: flags.playbook,
				User:     flags.user,
				Check:    flags.check,
				Diff:     flags.diff,
				Verbose:  flags.verbose,
			}
			for _, kv := range flags.vars {
				req.Vars = append(req.Vars, &pb.Var{
					Key:   kv.Key,
					Value: kv.Value,
				})
			}
			return req, nil
		},
		SendGRPCRequest: func(ctx context.Context, state *util.ExecuteState, req *pb.RunRequest) (<-chan *pb.RunManyResponse, error) {
			c := pb.NewPlaybookClientProxy(state.Conn)
			resp, err := c.RunOneMany(ctx, req)
			return resp, err
		},
		HandleSingleResp: func(ctx context.Context, state *util.ExecuteState, r *pb.RunManyResponse) subcommands.ExitStatus {
			out := state.Out[r.Index]
			err := state.Err[r.Index]

			fmt.Fprintf(out, "Target: %s (%d)\n\n", r.Target, r.Index)
			if r.Error != nil {
				fmt.Fprintf(err, "Ansible for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
				return subcommands.ExitFailure
			}
			fmt.Fprintf(out, "Return code: %d\nStdout:%s\nStderr:%s\n", r.Resp.ReturnCode, r.Resp.Stdout, r.Resp.Stderr)
			if r.Resp.ReturnCode != 0 {
				return subcommands.ExitFailure
			}
			return subcommands.ExitSuccess
		},
	}
	return client.NewCommandAdapter(&controller)
}
