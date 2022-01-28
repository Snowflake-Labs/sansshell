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

// Package main implements the SansShell CLI client.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	// Import the raw command clients you want, they automatically register
	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	_ "github.com/Snowflake-Labs/sansshell/services/ansible/client"
	_ "github.com/Snowflake-Labs/sansshell/services/exec/client"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/client"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/client"
	_ "github.com/Snowflake-Labs/sansshell/services/packages/client"
	_ "github.com/Snowflake-Labs/sansshell/services/process/client"
	_ "github.com/Snowflake-Labs/sansshell/services/service/client"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

var (
	defaultAddress = "localhost:50042"
	defaultOutput  = "-"
	defaultTimeout = 3 * time.Second

	proxyAddr  = flag.String("proxy", "", "Address to contact for proxy to sansshell-server. If blank a direct connection to the first entry in --targets will be made")
	timeout    = flag.Duration("timeout", defaultTimeout, "How long to wait for the command to complete")
	credSource = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	outputsDir = flag.String("output-dir", "", "If set defines a directory to emit output/errors from commands. Files will be generated based on target as destination/0 destination/0.error, etc.")

	// targets will be bound to --targets for sending a single request to N nodes.
	targetsFlag util.StringSliceFlag

	// outputs will be found to --outputs for directing output from a single request to N nodes.
	outputsFlag util.StringSliceFlag
)

func init() {
	targetsFlag.Set(defaultAddress)
	// Setup an empty slice so it can be deref'd below regardless of user input.
	outputsFlag.Target = &[]string{}

	flag.Var(&targetsFlag, "targets", "List of targets (separated by commas) to apply RPC against. If --proxy is not set must be one entry only.")
	flag.Var(&outputsFlag, "outputs", `List of output destinations (separated by commas) to direct output into.
    Use - to indicated stdout/stderr (default if nothing else is set). Using - does not have to be repeated per target.
	Errors will be emitted to <destination>.error separately from command/execution output which will be in the destination file.
	NOTE: This must map 1:1 with --targets except in the '-' case.`)

	subcommands.ImportantFlag("credential-source")
	subcommands.ImportantFlag("proxy")
	subcommands.ImportantFlag("targets")
	subcommands.ImportantFlag("outputs")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
}

func main() {
	flag.Parse()

	// Bunch of flag sanity checking
	if len(*targetsFlag.Target) == 0 {
		fmt.Fprintln(os.Stderr, "Must set --targets")
		os.Exit(1)
	}
	if len(*targetsFlag.Target) > 1 && *proxyAddr == "" {
		fmt.Fprintln(os.Stderr, "Can't set --targets to multiple entries without --proxy")
		os.Exit(1)
	}

	// Process combinations of outputs/output-dir that are valid and in the end
	// make sure outputsFlag has the correct relevant entries.
	if *outputsDir != "" {
		if len(*outputsFlag.Target) > 0 {
			fmt.Fprintln(os.Stderr, "Can't set --outputs and --output-dir at the same time.")
			os.Exit(1)
		}
		o, err := os.Stat(*outputsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't open %s: %v\n", *outputsDir, err)
			os.Exit(1)
		}
		if !o.Mode().IsDir() {
			fmt.Fprintf(os.Stderr, "%s: is not a directory\n", *outputsDir)
			os.Exit(1)
		}
		// Generate --outputs from --output-dir and the target count.
		for i := 0; i < len(*targetsFlag.Target); i++ {
			*outputsFlag.Target = append(*outputsFlag.Target, filepath.Join(*outputsDir, fmt.Sprintf("%d", i)))
		}
	} else {
		// No --outputs or --outputs-dir so we default to -. It'll process below.
		if len(*outputsFlag.Target) == 0 {
			*outputsFlag.Target = append(*outputsFlag.Target, defaultOutput)
		}

		if len(*outputsFlag.Target) != len(*targetsFlag.Target) {
			// Special case. We allow a single - and everything goes to stdout/stderr.
			if !(len(*outputsFlag.Target) == 1 && (*outputsFlag.Target)[0] == "-") {
				fmt.Fprintln(os.Stderr, "--outputs and --targets must contain the same number of entries")
				os.Exit(1)
			}
			if len(*outputsFlag.Target) > 1 && (*outputsFlag.Target)[0] == "-" {
				fmt.Fprintln(os.Stderr, "--outputs using '-' can only have one entry")
				os.Exit(1)
			}
			// Now if we have - passed we'll autofill it into the remaining slots for processing below.
			if (*outputsFlag.Target)[0] == "-" {
				for i := 1; i < len(*targetsFlag.Target); i++ {
					*outputsFlag.Target = append(*outputsFlag.Target, "-")
				}
			}
		}
	}
	ctx := context.Background()
	creds, err := mtls.LoadClientCredentials(ctx, *credSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load creds from %s - %v", *credSource, err)
		os.Exit(1)
	}

	// Set up a connection to the sansshell-server (possibly via proxy).
	conn, err := proxy.Dial(*proxyAddr, *targetsFlag.Target, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *targetsFlag.Target, err)
		os.Exit(1)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing connection - %v", err)
		}
	}()

	state := &util.ExecuteState{
		Conn: conn,
	}
	for _, out := range *outputsFlag.Target {
		if out == "-" {
			state.Out = append(state.Out, os.Stdout)
			state.Err = append(state.Err, os.Stderr)
			continue
		}
		file, err := os.Create(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't create output file %s - %v", out, err)
			os.Exit(1)
		}
		defer file.Close()
		errorFile := fmt.Sprintf("%s.error", out)
		errF, err := os.Create(errorFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't create error file file %s - %v", errorFile, err)
			os.Exit(1)
		}
		defer errF.Close()

		state.Out = append(state.Out, file)
		state.Err = append(state.Err, errF)
	}

	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	// Invoke the subcommand, passing the dialed connection object
	// TODO(jchacon): Pass a struct instead of 3 args.
	os.Exit(int(subcommands.Execute(ctx, state)))
}
