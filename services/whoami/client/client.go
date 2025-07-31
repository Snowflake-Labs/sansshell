/* Copyright (c) 2025 Snowflake Inc. All rights reserved.

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

package client

import (
	"context"
	"crypto/x509"
	"flag"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/go-logr/logr"
	"github.com/google/subcommands"
)

const (
	CredSourceFlags = "flags"
)

type WhoamiCommand struct{}

func (w *WhoamiCommand) Name() string             { return "whoami" }
func (w *WhoamiCommand) Synopsis() string         { return "Prints the current user and its groups" }
func (w *WhoamiCommand) Usage() string            { return "whoami\n" }
func (w *WhoamiCommand) SetFlags(f *flag.FlagSet) {}

func (w *WhoamiCommand) Execute(ctx context.Context, _ *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	logger := logr.FromContextOrDiscard(ctx)

	// Extract the credential source from args if provided
	var credSource string
	if len(args) > 0 {
		if v, ok := args[0].(string); ok {
			credSource = v
		}
	}

	if credSource == CredSourceFlags {
		logger.Info("loading new client cert from source", credSource)
		loader, err := mtls.Loader(credSource)
		if err != nil {
			fmt.Printf("failed to load corresponding certificate loader %v\n", err)
			return subcommands.ExitFailure
		}

		cert, err := loader.LoadClientCertificate(ctx)
		if err != nil {
			fmt.Printf("failed to load client certificate from %q: %v\n", credSource, err)
			return subcommands.ExitFailure
		}

		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			fmt.Printf("failed to parse certificate: %v", err)
			return subcommands.ExitFailure
		}

		ShowClientInfoFromCert(x509Cert)
		showGroupsFromCert(x509Cert)
		return subcommands.ExitSuccess
	} else {
		fmt.Printf("Unsupported credential source: %s\n", credSource)
		return subcommands.ExitFailure
	}
}
