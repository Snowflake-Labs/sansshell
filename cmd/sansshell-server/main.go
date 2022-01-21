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
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/server"

	// Import the server modules you want to expose, they automatically register
	_ "github.com/Snowflake-Labs/sansshell/services/ansible/server"
	_ "github.com/Snowflake-Labs/sansshell/services/exec/server"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/server"
	_ "github.com/Snowflake-Labs/sansshell/services/packages/server"
	_ "github.com/Snowflake-Labs/sansshell/services/process/server"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string
	hostport      = flag.String("hostport", "localhost:50042", "Where to listen for connections.")
	policyFlag    = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile    = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
	credSource    = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
)

// choosePolicy selects an OPA policy based on the flags, or calls log.Fatal if
// an invalid combination is provided.
func choosePolicy(logger logr.Logger) string {
	if *policyFlag != defaultPolicy && *policyFile != "" {
		logger.Error(errors.New("invalid policy flags"), "do not set both --policy and --policy-file")
		os.Exit(1)
	}

	var policy string
	if *policyFile != "" {
		pff, err := os.ReadFile(*policyFile)
		if err != nil {
			logger.Error(err, "os.ReadFile(policyFile)", "file", *policyFile)
		}
		logger.Info("using policy from --policy-file", "file", *policyFile)
		policy = string(pff)
	} else {
		if *policyFlag != defaultPolicy {
			logger.Info("using policy from --policy")
			policy = *policyFlag
		} else {
			logger.Info("using built-in policy")
			policy = defaultPolicy
		}
	}
	return policy
}

func main() {
	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-server")
	flag.Parse()

	policy := choosePolicy(logger)
	ctx := context.Background()

	creds, err := mtls.LoadServerCredentials(ctx, *credSource)
	if err != nil {
		logger.Error(err, "mtls.LoadServerCredentials", "credsource", *credSource)
		os.Exit(1)
	}

	if err := server.Serve(*hostport, creds, policy, logger); err != nil {
		logger.Error(err, "server.Serve", "hostport", *hostport)
		os.Exit(1)
	}
}
