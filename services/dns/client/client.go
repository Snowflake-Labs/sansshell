/* Copyright (c) 2022 Snowflake Inc. All rights reserved.

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

// Package client provides the client interface for 'dns'
package client

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/dns"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "dns"

func init() {
	subcommands.Register(&dnsCmd{}, subPackage)
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&lookupCmd{}, "")
	return c
}

type dnsCmd struct{}

func (*dnsCmd) Name() string { return subPackage }
func (p *dnsCmd) Synopsis() string {
	return client.GenerateSynopsis(setup(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *dnsCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*dnsCmd) SetFlags(f *flag.FlagSet) {}

func (p *dnsCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setup(f)
	return c.Execute(ctx, args...)
}

type lookupCmd struct{}

func (*lookupCmd) Name() string     { return "lookup" }
func (*lookupCmd) Synopsis() string { return "Executes a DNS lookup" }
func (*lookupCmd) Usage() string {
	return `lookup:
    Performs a simple DNS lookup for the provided hostname on the remote server.
`
}

func (p *lookupCmd) SetFlags(f *flag.FlagSet) {}

func (p *lookupCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Please specify a hostname to lookup.")
		return subcommands.ExitUsageError
	}

	c := pb.NewLookupClientProxy(state.Conn)

	req := &pb.LookupRequest{
		Hostname: f.Args()[0],
	}

	resp, err := c.LookupOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
		}
		return subcommands.ExitFailure
	}
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Dns lookup failure for target %s (%d) - error - %v\n", r.Target, r.Index, r.Error)
			continue
		}
		fmt.Fprint(state.Out[r.Index], strings.Join(r.Resp.Result, "\n"))
	}
	return subcommands.ExitSuccess

}
