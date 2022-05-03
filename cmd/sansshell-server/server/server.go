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
	"errors"
	"math/rand"
	"os"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/server"
	"github.com/go-logr/logr"
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
	// If non-zero don't exit immediately but instead sleep for 1..N random time.Duration
	// before bailing. This allows for credential loaders which may have temporary
	// problems to not cause a thundering herd of requests on mass death/restart
	// loops.
	JitterSleepOnError time.Duration
}

// Run takes the given context and RunState and starts up a sansshell server.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, rs RunState) {
	jitter := func() {
		var sleep time.Duration
		if rs.JitterSleepOnError > 0 {
			sleep = time.Duration(rand.Int63n(int64(rs.JitterSleepOnError))) + 1
			if sleep > rs.JitterSleepOnError {
				sleep = rs.JitterSleepOnError
			}
			rs.Logger.Error(errors.New("sleeping before exit"), "jitter", "sleep", sleep)
			time.Sleep(sleep)
		}
	}

	creds, err := mtls.LoadServerCredentials(ctx, rs.CredSource)
	if err != nil {
		rs.Logger.Error(err, "mtls.LoadServerCredentials", "credsource", rs.CredSource)
		jitter()
		os.Exit(1)
	}

	justificationHook := rpcauth.HookIf(rpcauth.JustificationHook(rs.JustificationFunc), func(input *rpcauth.RPCAuthInput) bool {
		return rs.Justification
	})
	if err := server.Serve(rs.Hostport, creds, rs.Policy, rs.Logger, justificationHook); err != nil {
		rs.Logger.Error(err, "server.Serve", "hostport", rs.Hostport)
		jitter()
		os.Exit(1)
	}
}
