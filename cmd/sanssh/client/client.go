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

// Package client provides functionality so that other uses of sansshell can provide their
// own main.go without having to cargo-cult everything across for common use cases.
// i.e. adding additional modules that are locally defined.
package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/google/subcommands"
	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/services/util"
)

func init() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
}

// RunState encapsulates all of the variable state needed
// to run a sansssh command.
type RunState struct {
	// Proxy is an optional proxy server to route requests.
	Proxy string
	// Targets is a list of remote targets to use when a proxy
	// is in use. For non proxy must be 1 entry.
	Targets []string
	// Outputs must map 1:1 with Targets indicating where to emit
	// output from commands. If the list is empty or a single entry
	// set to - then stdout/stderr will be used for all outputs.
	Outputs []string
	// OutputsDir defines a directory to place outputs instead of
	// specifying then in Outputs. The files will be names 0.output,
	// 1.output and .error respectively for each target.
	OutputsDir string
	// CredSource is a registered credential source with the mtls package.
	CredSource string
	// Timeout is the duration to place on the context when making RPC calls.
	Timeout time.Duration
}

const (
	defaultOutput = "-"
)

// Run takes the given context and RunState and executes the command passed in after parsing with flags.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, rs RunState) {
	// Bunch of flag sanity checking
	if len(rs.Targets) == 0 && rs.Proxy == "" {
		fmt.Fprintln(os.Stderr, "Must set a target or a proxy")
		os.Exit(1)
	}
	if len(rs.Targets) > 1 && rs.Proxy == "" {
		fmt.Fprintln(os.Stderr, "Can't set targets to multiple entries without a proxy")
		os.Exit(1)
	}

	// Process combinations of outputs/output-dir that are valid and in the end
	// make sure outputsFlag has the correct relevant entries.
	if rs.OutputsDir != "" {
		if len(rs.Outputs) > 0 {
			fmt.Fprintln(os.Stderr, "Can't set outputs and output-dir at the same time.")
			os.Exit(1)
		}
		o, err := os.Stat(rs.OutputsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't open %s: %v\n", rs.OutputsDir, err)
			os.Exit(1)
		}
		if !o.Mode().IsDir() {
			fmt.Fprintf(os.Stderr, "%s: is not a directory\n", rs.OutputsDir)
			os.Exit(1)
		}
		// Generate outputs from output-dir and the target count.
		for i := range rs.Targets {
			rs.Outputs = append(rs.Outputs, filepath.Join(rs.OutputsDir, fmt.Sprintf("%d", i)))
		}
	} else {
		// No outputs or outputs-dir so we default to -. It'll process below.
		if len(rs.Outputs) == 0 {
			rs.Outputs = append(rs.Outputs, defaultOutput)
		}

		if len(rs.Outputs) != len(rs.Targets) {
			// Special case. We allow a single - and everything goes to stdout/stderr.
			if !(len(rs.Outputs) == 1 && rs.Outputs[0] == defaultOutput) {
				fmt.Fprintln(os.Stderr, "outputs and targets must contain the same number of entries")
				os.Exit(1)
			}
			if len(rs.Outputs) > 1 && rs.Outputs[0] == defaultOutput {
				fmt.Fprintf(os.Stderr, "outputs using %q can only have one entry\n", defaultOutput)
				os.Exit(1)
			}
			// Now if we have - passed we'll autofill it into the remaining slots for processing below.
			if rs.Outputs[0] == defaultOutput {
				for i := 1; i < len(rs.Targets); i++ {
					rs.Outputs = append(rs.Outputs, defaultOutput)
				}
			}
		}
	}
	creds, err := mtls.LoadClientCredentials(ctx, rs.CredSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load creds from %s - %v\n", rs.CredSource, err)
		os.Exit(1)
	}

	// Set up a connection to the sansshell-server (possibly via proxy).
	conn, err := proxy.Dial(rs.Proxy, rs.Targets, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to proxy %q node(s) %v: %v\n", rs.Proxy, rs.Targets, err)
		os.Exit(1)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing connection - %v\n", err)
		}
	}()

	state := &util.ExecuteState{
		Conn: conn,
	}
	for _, out := range rs.Outputs {
		if out == "-" {
			state.Out = append(state.Out, os.Stdout)
			state.Err = append(state.Err, os.Stderr)
			continue
		}
		file, err := os.Create(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't create output file %s - %v\n", out, err)
			os.Exit(1)
		}
		defer file.Close()
		errorFile := fmt.Sprintf("%s.error", out)
		errF, err := os.Create(errorFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't create error file file %s - %v\n", errorFile, err)
			os.Exit(1)
		}
		defer errF.Close()

		state.Out = append(state.Out, file)
		state.Err = append(state.Err, errF)
	}

	ctx, cancel := context.WithTimeout(ctx, rs.Timeout)
	defer cancel()

	// Invoke the subcommand, passing the dialed connection object
	// TODO(jchacon): Pass a struct instead of 3 args.
	os.Exit(int(subcommands.Execute(ctx, state)))
}
