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
	"strings"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/cmd/sanssh/client"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
	"google.golang.org/grpc/metadata"

	// Import services here to make them accessible for CLI
	_ "github.com/Snowflake-Labs/sansshell/services/ansible/client"
	_ "github.com/Snowflake-Labs/sansshell/services/exec/client"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/client"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/client"
	_ "github.com/Snowflake-Labs/sansshell/services/packages/client"
	_ "github.com/Snowflake-Labs/sansshell/services/process/client"
	_ "github.com/Snowflake-Labs/sansshell/services/sansshell/client"
	_ "github.com/Snowflake-Labs/sansshell/services/service/client"
)

var (
	defaultAddress = "localhost:50042"
	defaultTimeout = 3 * time.Second

	proxyAddr     = flag.String("proxy", "", "Address to contact for proxy to sansshell-server. If blank a direct connection to the first entry in --targets will be made")
	timeout       = flag.Duration("timeout", defaultTimeout, "How long to wait for the command to complete")
	credSource    = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	outputsDir    = flag.String("output-dir", "", "If set defines a directory to emit output/errors from commands. Files will be generated based on target as destination/0 destination/0.error, etc.")
	justification = flag.String("justification", "", "If non-empty will add the key '"+rpcauth.ReqJustKey+"' to the outgoing context Metadata to be passed along to the server for possbile validation and logging.")

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
	subcommands.ImportantFlag("output-dir")
	subcommands.ImportantFlag("justification")
}

func main() {
	flag.Parse()

	rs := client.RunState{
		Proxy:      *proxyAddr,
		Targets:    *targetsFlag.Target,
		Outputs:    *outputsFlag.Target,
		OutputsDir: *outputsDir,
		CredSource: *credSource,
		Timeout:    *timeout,
	}
	ctx := context.Background()
	if *justification != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, rpcauth.ReqJustKey, *justification)
	}
	client.Run(ctx, rs)
}
