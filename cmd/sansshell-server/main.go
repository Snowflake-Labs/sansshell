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

	"github.com/go-logr/stdr"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/cmd/sansshell-server/server"
	"github.com/Snowflake-Labs/sansshell/cmd/util"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string

	policyFlag = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
	hostport   = flag.String("hostport", "localhost:50042", "Where to listen for connections.")
	credSource = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	verbosity  = flag.Int("v", 0, "Verbosity level. > 0 indicates more extensive logging")
)

func main() {
	flag.Parse()

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-server")
	stdr.SetVerbosity(*verbosity)

	// TODO(jallie): implement the ability to 'hot reload' policy, since
	// that could likely be done underneath the authorizer, with little
	// disruption to existing connections.
	policy := util.ChoosePolicy(logger, defaultPolicy, *policyFlag, *policyFile)
	ctx := context.Background()

	rs := server.RunState{
		Logger:     logger,
		CredSource: *credSource,
		Hostport:   *hostport,
		Policy:     policy,
	}
	server.Run(ctx, rs)
}
