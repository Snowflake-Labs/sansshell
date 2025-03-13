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

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	cli_controllers "github.com/Snowflake-Labs/sansshell/services/ansible/client/cli-controllers"
)

const subPackage = "ansible"

func init() {
	subcommands.Register(&ansibleCmd{}, subPackage)
}

func (*ansibleCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&cli_controllers.PlaybookCmd{}, "")
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
