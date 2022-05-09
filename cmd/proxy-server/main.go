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
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/cmd/proxy-server/server"
	"github.com/Snowflake-Labs/sansshell/cmd/util"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"

	// Import services here to make them proxy-able
	_ "github.com/Snowflake-Labs/sansshell/services/ansible"
	_ "github.com/Snowflake-Labs/sansshell/services/exec"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile"
	_ "github.com/Snowflake-Labs/sansshell/services/packages"
	_ "github.com/Snowflake-Labs/sansshell/services/process"
	ssserver "github.com/Snowflake-Labs/sansshell/services/sansshell/server"
	_ "github.com/Snowflake-Labs/sansshell/services/service"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string

	policyFlag       = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile       = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
	clientPolicyFlag = flag.String("client-policy", "", "OPA policy for outbound client actions (i.e. connecting to sansshell servers). If empty no policy is applied.")
	clientPolicyFile = flag.String("client-policy-file", "", "Path to a file with a client OPA.  If empty uses --client-policy")
	hostport         = flag.String("hostport", "localhost:50043", "Where to listen for connections.")
	credSource       = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS creds (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	verbosity        = flag.Int("v", 0, "Verbosity level. > 0 indicates more extensive logging")
	validate         = flag.Bool("validate", false, "If true will evaluate the policy and then exit (non-zero on error)")
	justification    = flag.Bool("justification", false, "If true then justification (which is logged and possibly validated) must be passed along in the client context Metadata with the key '"+rpcauth.ReqJustKey+"'")
	version          bool
)

func init() {
	flag.BoolVar(&version, "version", false, "Returns the server built version from the sansshell server package")
}

func main() {
	flag.Parse()

	if version {
		fmt.Printf("Version: %s\n", ssserver.Version)
		os.Exit(0)
	}

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-proxy")
	stdr.SetVerbosity(*verbosity)

	policy := util.ChoosePolicy(logger, defaultPolicy, *policyFlag, *policyFile)
	clientPolicy := util.ChoosePolicy(logger, "", *clientPolicyFlag, *clientPolicyFile)
	ctx := logr.NewContext(context.Background(), logger)

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
		ClientPolicy:  clientPolicy,
		CredSource:    *credSource,
		Hostport:      *hostport,
		Justification: *justification,
	}
	server.Run(ctx, rs)
}
