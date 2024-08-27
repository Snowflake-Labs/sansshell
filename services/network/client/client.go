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

// Package client provides the client interface for 'network'
package client

import (
	"context"
	"flag"
	"github.com/Snowflake-Labs/sansshell/services/network/client/infrastructure/input"
	"github.com/google/subcommands"

	sansshellClient "github.com/Snowflake-Labs/sansshell/client"
)

const subPackage = "network"

func init() {
	subcommands.Register(&networkCmd{}, subPackage)
}

type networkCmd struct{}

func (*networkCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := sansshellClient.SetupSubpackage(subPackage, f)
	c.Register(input.NewTCPCheckCmd(), "")
	return c
}

func (*networkCmd) Name() string { return subPackage }
func (p *networkCmd) Synopsis() string {
	return sansshellClient.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *networkCmd) Usage() string {
	return sansshellClient.GenerateUsage(subPackage, p.Synopsis())
}
func (*networkCmd) SetFlags(f *flag.FlagSet) {}

func (p *networkCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}
