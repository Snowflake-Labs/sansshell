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
	"flag"
	"fmt"

	"github.com/google/subcommands"
)

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
// with N sub-commands contained within it.
func GenerateSynopsis(c *subcommands.Commander) string {
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

// GenerateUsage will return a usage string to a top level command with N
// sub-commands contained within it.
func GenerateUsage(name string, synopsis string) string {
	return fmt.Sprintf("%s has several subcommands. Pick one to perform the action you wish:\n%s", name, synopsis)
}
