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

package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"github.com/Snowflake-Labs/sansshell/cmd/proxy-server/server"
	"github.com/Snowflake-Labs/sansshell/cmd/util"
	"github.com/Snowflake-Labs/sansshell/telemetry"
	"github.com/go-logr/stdr"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string

	policyFlag    = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile    = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
	hostport      = flag.String("hostport", "localhost:50043", "Where to listen for connections.")
	credSource    = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS creds (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	verbosity     = flag.Int("v", 0, "Verbosity level. > 0 indicates more extensive logging")
	validate      = flag.Bool("validate", false, "If true will evaluate the policy and then exit (non-zero on error)")
	justification = flag.Bool("justification", false, "If true then justification (which is logged) must be passed along in the client context Metadata with the key "+telemetry.ReqJustKey)
)

func main() {
	flag.Parse()

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-proxy")
	stdr.SetVerbosity(*verbosity)

	policy := util.ChoosePolicy(logger, defaultPolicy, *policyFlag, *policyFile)
	ctx := context.Background()

	if *validate {
		_, err := opa.NewAuthzPolicy(ctx, policy)
		if err != nil {
			log.Fatalf("Invalid policy: %v\n", err)
		}
		fmt.Println("Policy passes.")
		os.Exit(0)
	}

	rs := server.RunState{
		Logger:        logger,
		Policy:        policy,
		CredSource:    *credSource,
		Hostport:      *hostport,
		Justification: *justification,
	}
	server.Run(ctx, rs)
}
