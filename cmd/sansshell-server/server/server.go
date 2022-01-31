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

// Package server provides functioanlity so that other uses of sansshell can provide their
// own main.go without having to cargo-cult everything across for common use cases.
// i.e. adding additional modules that are locally defined.
package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/server"
	"github.com/go-logr/logr"

	// Import the server modules you want to expose, they automatically register
	_ "github.com/Snowflake-Labs/sansshell/services/ansible/server"
	_ "github.com/Snowflake-Labs/sansshell/services/exec/server"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/server"
	_ "github.com/Snowflake-Labs/sansshell/services/packages/server"
	_ "github.com/Snowflake-Labs/sansshell/services/process/server"
	_ "github.com/Snowflake-Labs/sansshell/services/service/server"
)

var (
	hostport   = flag.String("hostport", "localhost:50042", "Where to listen for connections.")
	credSource = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
)

// Run takes the given context, logger and policy and starts up a sansshell server using the flags above
// to provide credentials.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, logger logr.Logger, policy string) {
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
