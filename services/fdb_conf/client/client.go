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
	"os"

	"fmt"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb_conf"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
)

const subPackage = "fdb_conf"

func init() {
	subcommands.Register(&fdbConfCmd{}, subPackage)
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&fdbConfReadCmd{}, "")
	c.Register(&fdbConfWriteCmd{}, "")
	c.Register(&fdbConfDeleteCmd{}, "")

	return c
}

type fdbConfCmd struct{}

func (*fdbConfCmd) Name() string             { return subPackage }
func (*fdbConfCmd) SetFlags(_ *flag.FlagSet) {}
func (*fdbConfCmd) Synopsis() string {
	return client.GenerateSynopsis(setup(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *fdbConfCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}

func (p *fdbConfCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setup(f)
	return c.Execute(ctx, args...)
}

type fdbConfReadCmd struct {
	path string
}

func (*fdbConfReadCmd) Name() string { return "read" }
func (*fdbConfReadCmd) Synopsis() string {
	return "Read value from a section for a given key and return a response."
}
func (*fdbConfReadCmd) Usage() string {
	return `read <section> <key> [--path <config>]:
Read a key from a section specified in a FDB config file.

Default location for config file is /etc/foundationdb/foundationdb.conf.
`
}

func (r *fdbConfReadCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&r.path, "path", "/etc/foundationdb/foundationdb.conf", "The absolute path to FDB config.")
}

func (r *fdbConfReadCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewFdbConfClientProxy(state.Conn)

	if len(f.Args()) != 2 {
		fmt.Fprint(os.Stderr, "invalid number of parameters: specify section and key only")
		return subcommands.ExitFailure
	}

	section := f.Args()[0]
	key := f.Args()[1]

	resp, err := c.ReadOneMany(ctx, &pb.ReadRequest{Location: &pb.Location{Section: section, Key: key, File: r.path}})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "fdb config read error: %v\n", err)
		}

		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "fdb config read error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}

type fdbConfWriteCmd struct {
	path string
}

func (*fdbConfWriteCmd) Name() string { return "write" }
func (*fdbConfWriteCmd) Synopsis() string {
	return "Write a key-value pair to a section of an FDB config."
}

func (*fdbConfWriteCmd) Usage() string {
	return `write <section> <key> <value> [--path <config>]:
Write a key-value pair to a section specified in a FDB config file.

Default location for config file is /etc/foundationdb/foundationdb.conf.
`
}

func (w *fdbConfWriteCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&w.path, "path", "/etc/foundationdb/foundationdb.conf", "The absolute path to FDB config.")
}

func (w *fdbConfWriteCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewFdbConfClientProxy(state.Conn)

	if len(f.Args()) != 3 {
		fmt.Fprint(os.Stderr, "invalid number of parameters: specify section, key and value only")
		return subcommands.ExitFailure
	}

	section, key, value := f.Args()[0], f.Args()[1], f.Args()[2]

	resp, err := c.WriteOneMany(ctx, &pb.WriteRequest{
		Location: &pb.Location{Section: section, Key: key, File: w.path},
		Value:    value,
	})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "fdb config write error: %v\n", err)
		}

		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "fdb config write error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}

type fdbConfDeleteCmd struct {
	path          string
	deleteSection bool
}

func (*fdbConfDeleteCmd) Name() string { return "delete" }
func (*fdbConfDeleteCmd) Synopsis() string {
	return "Delete key from a section or entire section"
}

func (*fdbConfDeleteCmd) Usage() string {
	return `delete [--delete-section] <section> [<key>]:
Delete key from a section or entire section.
When entire section is deleted, '--delete-section' flag is mandatory but 'key' is optional.

Default location for config file is /etc/foundationdb/foundationdb.conf.
`
}

func (d *fdbConfDeleteCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&d.path, "path", "/etc/foundationdb/foundationdb.conf", "The absolute path to FDB config.")
	f.BoolVar(&d.deleteSection, "delete-section", false, "Delete section safeguard.")
}

func (d *fdbConfDeleteCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewFdbConfClientProxy(state.Conn)

	if len(f.Args()) < 1 {
		fmt.Fprint(os.Stderr, "invalid number of parameters: specify section name.")
		return subcommands.ExitFailure
	}

	if len(f.Args()) > 2 {
		fmt.Fprint(os.Stderr, "invalid number of parameters: specify section name and optional key only.")
		return subcommands.ExitFailure
	}

	section := f.Args()[0]

	var key string
	if len(f.Args()) == 2 {
		key = f.Args()[1]
	}

	if key == "" && !d.deleteSection {
		fmt.Fprint(os.Stderr, "invalid parameters: `delete-section` must be set if `key` is empty.")
		return subcommands.ExitFailure
	}

	if key != "" && d.deleteSection {
		fmt.Fprint(os.Stderr, "invalid parameters: `delete-section` and `key` are mutually exclusive.")
		return subcommands.ExitFailure
	}

	resp, err := c.DeleteOneMany(ctx, &pb.DeleteRequest{Location: &pb.Location{Section: section, Key: key, File: d.path}})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "fdb config delete error: %v\n", err)
		}

		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "fdb config delete error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}
