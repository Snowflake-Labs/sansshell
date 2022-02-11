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

// Package server provides functionality so that other uses of sansshell can provide their
// own main.go without having to cargo-cult everything across for common use cases.
// i.e. adding additional modules that are locally defined.
package server

import (
	"context"
	"os"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
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

type RunState struct {
	// Logger is used for all logging.
	Logger logr.Logger
	// CredSource is a registered credential source with the mtls package.
	CredSource string
	// Hostport is the host:port to run the server.
	Hostport string
	// Policy is an OPA policy for determining authz decisions.
	Policy string
	// Justification if true requires justification to be set in the
	// incoming RPC context Metadata (to the key defined in the telemetry package).
	Justification bool
	// JustificationFunc will be called if Justication is true and a justification
	// entry is found. The supplied function can then do any validation it wants
	// in order to ensure it's compliant.
	JustificationFunc func(string) error
}

// Run takes the given context and RunState and starts up a sansshell server.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, rs RunState) {
	creds, err := mtls.LoadServerCredentials(ctx, rs.CredSource)
	if err != nil {
		rs.Logger.Error(err, "mtls.LoadServerCredentials", "credsource", rs.CredSource)
		os.Exit(1)
	}

	if err := server.Serve(rs.Hostport, creds, rs.Policy, rs.Logger, rs.Justification, rs.JustificationFunc); err != nil {
		rs.Logger.Error(err, "server.Serve", "hostport", rs.Hostport)
		os.Exit(1)
	}
}
