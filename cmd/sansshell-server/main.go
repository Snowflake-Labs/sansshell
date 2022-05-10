//go:build go1.17
// +build go1.17

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

// Package main implements the SansShell server.
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"

	_ "gocloud.dev/blob/azureblob" // Pull in Azure blob support
	_ "gocloud.dev/blob/fileblob"  // Pull in file blob support
	_ "gocloud.dev/blob/gcsblob"   // Pull in GCS blob support
	_ "gocloud.dev/blob/s3blob"    // Pull in S3 blob support

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/cmd/sansshell-server/server"
	"github.com/Snowflake-Labs/sansshell/cmd/util"

	// Import the server modules you want to expose, they automatically register
	_ "github.com/Snowflake-Labs/sansshell/services/ansible/server"
	_ "github.com/Snowflake-Labs/sansshell/services/exec/server"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/server"
	_ "github.com/Snowflake-Labs/sansshell/services/packages/server"
	_ "github.com/Snowflake-Labs/sansshell/services/process/server"
	ssserver "github.com/Snowflake-Labs/sansshell/services/sansshell/server"
	_ "github.com/Snowflake-Labs/sansshell/services/service/server"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string

	policyFlag    = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile    = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
	hostport      = flag.String("hostport", "localhost:50042", "Where to listen for connections.")
	credSource    = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	verbosity     = flag.Int("v", 0, "Verbosity level. > 0 indicates more extensive logging")
	validate      = flag.Bool("validate", false, "If true will evaluate the policy and then exit (non-zero on error)")
	justification = flag.Bool("justification", false, "If true then justification (which is logged and possibly validated) must be passed along in the client context Metadata with the key '"+rpcauth.ReqJustKey+"'")
	version       bool
)

func init() {
	flag.BoolVar(&version, "version", false, "Returns the server built version from the sansshell server package")

	// An example of how to override the flag default for a specific implementation
	//f := flag.Lookup("ansible_playbook_bin")
	//f.DefValue = "/foo/ansible"
}

func main() {
	flag.Parse()

	if version {
		fmt.Printf("Version: %s\n", ssserver.Version)
		os.Exit(0)
	}

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-server")
	stdr.SetVerbosity(*verbosity)

	policy := util.ChoosePolicy(logger, defaultPolicy, *policyFlag, *policyFile)
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
		CredSource:    *credSource,
		Hostport:      *hostport,
		Policy:        policy,
		Justification: *justification,
	}
	server.Run(ctx, rs)
}
