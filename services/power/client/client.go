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

// Package client provides the client interface for 'power'
package client

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Snowflake-Labs/sansshell/client"
	"github.com/google/subcommands"

	pb "github.com/Snowflake-Labs/sansshell/services/power"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "power"

func init() {
	subcommands.Register(&powerCmd{}, subPackage)
}

type powerCmd struct{}

func (*powerCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&rebootCmd{}, "")
	return c
}

func (*powerCmd) Name() string { return subPackage }
func (p *powerCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *powerCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}

func (*powerCmd) SetFlags(f *flag.FlagSet) {}

func (p *powerCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

type rebootCmd struct {
	when    string
	message string
}

func (*rebootCmd) Name() string { return "reboot" }
func (*rebootCmd) Synopsis() string {
	return `Reboot a host at a predefined time.`
}
func (a *rebootCmd) Usage() string {
	return client.GenerateUsage(subPackage, a.Synopsis())
}

func (a *rebootCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&a.when, "when", "now", "When to reboot a host. Valid values are 'now', 'hh:mm', '+5'.")
	f.StringVar(&a.message, "message", "", "Reboot message.")
}

func (a *rebootCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if a.when == "" {
		fmt.Fprintln(os.Stderr, "--when must be filled in")
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)

	c := pb.NewPowerClientProxy(state.Conn)

	req := &pb.RebootRequest{
		When:    a.when,
		Message: a.message,
	}

	resp, err := c.RebootOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - Run returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		fmt.Fprintf(state.Out[r.Index], "Target: %s (%d)\n\n", r.Target, r.Index)
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Reboot for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			// If any target had errors it needs to be reported for that target but we still
			// need to process responses off the channel. Final return code though should
			// indicate something failed.
			retCode = subcommands.ExitFailure
			continue
		}
	}

	return retCode
}
