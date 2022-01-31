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
	"errors"
	"flag"
	"log"
	"os"

	"github.com/Snowflake-Labs/sansshell/cmd/proxy-server/server"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string

	policyFlag = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
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
	flag.Parse()

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-proxy")

	ctx := context.Background()

	// TODO(jallie): implement the ability to 'hot reload' policy, since
	// that could likely be done underneath the authorizer, with little
	// disruption to existing connections.
	policy := choosePolicy(logger)

	server.Run(ctx, logger, policy)
}
