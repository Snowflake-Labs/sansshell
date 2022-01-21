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
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/subcommands"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

func init() {
	subcommands.Register(&healthcheckCmd{}, "healthcheck")
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := subcommands.NewCommander(f, "process")
	c.Register(&validateCmd{}, "")
	c.Register(c.HelpCommand(), "")
	c.Register(c.FlagsCommand(), "")
	c.Register(c.CommandsCommand(), "")
	return c
}

type healthcheckCmd struct{}

func (*healthcheckCmd) Name() string { return "healthcheck" }
func (p *healthcheckCmd) Synopsis() string {
	c := setup(flag.NewFlagSet("", flag.ContinueOnError))
	b := &bytes.Buffer{}
	b.WriteString("\n")
	fn := func(c *subcommands.CommandGroup, comm subcommands.Command) {
		switch comm.Name() {
		case "help", "flags", "commands":
			break
		default:
			fmt.Fprintf(b, "\t\t%s\t- %s\n", comm.Name(), comm.Synopsis())
		}
	}
	c.VisitCommands(fn)
	return b.String()
}
func (p *healthcheckCmd) Usage() string {
	return "health has several subcommands. Pick one to perform the action you wish\n" + p.Synopsis()
}
func (*healthcheckCmd) SetFlags(f *flag.FlagSet) {}

func (p *healthcheckCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setup(f)
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

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.OkOneMany(ctx, &emptypb.Empty{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not healthcheck server: %v\n", err)
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Out[r.Index], "Healthcheck for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}
