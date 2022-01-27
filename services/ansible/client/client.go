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
	"os"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/ansible"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "ansible"

func init() {
	subcommands.Register(&ansibleCmd{}, subPackage)
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&playbookCmd{}, "")
	return c
}

type ansibleCmd struct{}

func (*ansibleCmd) Name() string { return subPackage }
func (p *ansibleCmd) Synopsis() string {
	return client.GenerateSynopsis(setup(flag.NewFlagSet("", flag.ContinueOnError)))
}
func (p *ansibleCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*ansibleCmd) SetFlags(f *flag.FlagSet) {}

func (p *ansibleCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setup(f)
	return c.Execute(ctx, args...)
}

type playbookCmd struct {
	playbook string
	vars     util.KeyValueSliceFlag
	user     string
	check    bool
	diff     bool
	verbose  bool
}

func (*playbookCmd) Name() string     { return "playbook" }
func (*playbookCmd) Synopsis() string { return "Run an ansible playbook on the server." }
func (*playbookCmd) Usage() string {
	return `ansible:
  Run an ansible playbook on the remote server.
`
}

func (a *playbookCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&a.playbook, "playbook", "", "The absolute path to the playbook to execute on the remote server.")
	f.Var(&a.vars, "vars", "Pass key=value (via -e) to ansible-playbook. Multiple values can be specified separated by commas")
	f.StringVar(&a.user, "user", "", "Run the playbook as this user")
	f.BoolVar(&a.check, "check", false, "If true the playbook will be run with --check passed as an argument")
	f.BoolVar(&a.diff, "diff", false, "If true the playbook will be run with --diff passed as an argument")
	f.BoolVar(&a.verbose, "verbose", false, "If true the playbook wiill be run with -vvv passed as an argument")
}

func (a *playbookCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if a.playbook == "" {
		fmt.Fprintln(os.Stderr, "--playbook is required")
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)

	c := pb.NewPlaybookClientProxy(state.Conn)

	req := &pb.RunRequest{
		Playbook: a.playbook,
		User:     a.user,
		Check:    a.check,
		Diff:     a.diff,
		Verbose:  a.verbose,
	}
	for _, kv := range a.vars {
		req.Vars = append(req.Vars, &pb.Var{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}

	resp, err := c.RunOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Run returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		fmt.Fprintf(state.Out[r.Index], "Target: %s (%d)\n\n", r.Target, r.Index)
		if r.Error != nil {
			fmt.Fprintf(state.Out[r.Index], "Ansible for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprintf(state.Out[r.Index], "Return code: %d\nStdout:%s\nStderr:%s\n", r.Resp.ReturnCode, r.Resp.Stdout, r.Resp.Stderr)
	}
	return retCode
}
