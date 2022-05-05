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
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/cmd/sanssh/client"
	cmdUtil "github.com/Snowflake-Labs/sansshell/cmd/util"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
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

const (
	defaultProxyPort  = 50043
	defaultTargetPort = 50042
	proxyEnv          = "SANSSHELL_PROXY"
)

var (
	defaultTimeout = 30 * time.Second

	proxyAddr = flag.String("proxy", "", fmt.Sprintf(
		`Address (host[:port]) to contact for proxy to sansshell-server.
%s in the environment can also be set instead of setting this flag. The flag will take precedence.
If blank a direct connection to the first entry in --targets will be made.
If port is blank the default of %d will be used`, proxyEnv, defaultProxyPort))
	timeout          = flag.Duration("timeout", defaultTimeout, "How long to wait for the command to complete")
	credSource       = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	outputsDir       = flag.String("output-dir", "", "If set defines a directory to emit output/errors from commands. Files will be generated based on target as destination/0 destination/0.error, etc.")
	justification    = flag.String("justification", "", "If non-empty will add the key '"+rpcauth.ReqJustKey+"' to the outgoing context Metadata to be passed along to the server for possible validation and logging.")
	targetsFile      = flag.String("targets-file", "", "If set read the targets list line by line (as host[:port]) from the indicated file instead of using --targets (error if both flags are used). A blank port acts the same as --targets")
	clientPolicyFlag = flag.String("client-policy", "", "OPA policy for outbound client actions.  If empty no policy is applied.")
	clientPolicyFile = flag.String("client-policy-file", "", "Path to a file with a client OPA.  If empty uses --client-policy")
	verbosity        = flag.Int("v", 0, "Verbosity level. > 0 indicates more extensive logging")

	// targets will be bound to --targets for sending a single request to N nodes.
	targetsFlag util.StringSliceFlag

	// outputs will be found to --outputs for directing output from a single request to N nodes.
	outputsFlag util.StringSliceFlag
)

func init() {
	// Setup an empty slice so it can be deref'd below regardless of user input.
	outputsFlag.Target = &[]string{}
	targetsFlag.Target = &[]string{}

	flag.Var(&targetsFlag, "targets", fmt.Sprintf("List of targets (host[:port] separated by commas) to apply RPC against. If --proxy is not set must be one entry only. If port is blank the default of %d will be used", defaultTargetPort))
	flag.Var(&outputsFlag, "outputs", `List of output destinations (separated by commas) to direct output into.
    Use - to indicated stdout/stderr (default if nothing else is set). Using - does not have to be repeated per target.
	Errors will be emitted to <destination>.error separately from command/execution output which will be in the destination file.
	NOTE: This must map 1:1 with --targets except in the '-' case.`)

	subcommands.ImportantFlag("credential-source")
	subcommands.ImportantFlag("proxy")
	subcommands.ImportantFlag("targets")
	subcommands.ImportantFlag("outputs")
	subcommands.ImportantFlag("output-dir")
	subcommands.ImportantFlag("targets-file")
	subcommands.ImportantFlag("justification")
	subcommands.ImportantFlag("client-policy")
	subcommands.ImportantFlag("client-policy-file")
	subcommands.ImportantFlag("v")
}

func hasPort(s string) bool {
	// Returns true if the provided address does not include a port number.
	return strings.LastIndex(s, "]") < strings.LastIndex(s, ":")
}

func main() {
	// If this is blank it'll remain blank which is fine
	// as that means just talk to --targets[0] instead.
	// If the flag itself was set that will override.
	*proxyAddr = os.Getenv(proxyEnv)
	flag.Parse()

	// If we're given a --targets-file read it in and stuff into targetsFlag
	// so it can be processed below as if it was set that way.
	if *targetsFile != "" {
		// Can't set both flags.
		if len(*targetsFlag.Target) > 0 {
			log.Fatal("can't set --targets-file and --targets at the same time")
		}
		f, err := os.Open(*targetsFile)
		if err != nil {
			log.Fatalf("can't open %s: %v", *targetsFile, err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			*targetsFlag.Target = append(*targetsFlag.Target, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("scanning error reading %s: %v", *targetsFile, err)
		}
	}

	// Add the default proxy port (if needed).
	if *proxyAddr != "" && !hasPort(*proxyAddr) {
		*proxyAddr = fmt.Sprintf("%s:%d", *proxyAddr, defaultProxyPort)
	}
	// Add default target port (if needed).
	for i, t := range *targetsFlag.Target {
		if !hasPort(t) {
			(*targetsFlag.Target)[i] = fmt.Sprintf("%s:%d", t, defaultTargetPort)
		}
	}
	clientPolicy := cmdUtil.ChoosePolicy(logr.Discard(), "", *clientPolicyFlag, *clientPolicyFile)

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanssh")
	stdr.SetVerbosity(*verbosity)

	rs := client.RunState{
		Proxy:        *proxyAddr,
		Targets:      *targetsFlag.Target,
		Outputs:      *outputsFlag.Target,
		OutputsDir:   *outputsDir,
		CredSource:   *credSource,
		Timeout:      *timeout,
		ClientPolicy: clientPolicy,
	}
	ctx := logr.NewContext(context.Background(), logger)

	if *justification != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, rpcauth.ReqJustKey, *justification)
	}
	client.Run(ctx, rs)
}
