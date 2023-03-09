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

// Package client provides the client interface for 'healthcheck'
package client

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/subcommands"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "healthcheck"

func init() {
	subcommands.Register(&healthcheckCmd{}, subPackage)
}

func (*healthcheckCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&validateCmd{}, "")
	return c
}

type healthcheckCmd struct{}

func (*healthcheckCmd) Name() string { return subPackage }
func (p *healthcheckCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *healthcheckCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*healthcheckCmd) SetFlags(f *flag.FlagSet) {}

func (p *healthcheckCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

type validateCmd struct{}

func (*validateCmd) Name() string     { return "validate" }
func (*validateCmd) Synopsis() string { return "Confirm connectivity to working server." }
func (*validateCmd) Usage() string {
	return `healthcheck:
  Sends an empty request and expects an empty response.  Only prints errors.
`
}

func (p *validateCmd) SetFlags(f *flag.FlagSet) {}

func (p *validateCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewHealthCheckClientProxy(state.Conn)

	resp, err := c.OkOneMany(ctx, &emptypb.Empty{})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not healthcheck servers: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Healthcheck for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			// If any target had errors it needs to be reported for that target but we still
			// need to process responses off the channel. Final return code though should
			// indicate something failed.
			retCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprintf(state.Out[r.Index], "Target %s (%d) healthy\n", r.Target, r.Index)
	}
	return retCode
}
