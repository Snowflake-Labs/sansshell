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
	"log"
	"os"

	"github.com/Snowflake-Labs/sansshell/cmd/proxy-server/server"
	"github.com/Snowflake-Labs/sansshell/cmd/util"
	"github.com/go-logr/stdr"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string

	policyFlag = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
)

func main() {
	flag.Parse()

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-proxy")

	ctx := context.Background()

	// TODO(jallie): implement the ability to 'hot reload' policy, since
	// that could likely be done underneath the authorizer, with little
	// disruption to existing connections.
	policy := util.ChoosePolicy(logger, defaultPolicy, *policyFlag, *policyFile)

	server.Run(ctx, logger, policy)
}
