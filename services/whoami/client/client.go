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
	"flag"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/google/subcommands"
)

type WhoamiCommand struct{}

func (w *WhoamiCommand) Name() string             { return "whoami" }
func (w *WhoamiCommand) Synopsis() string         { return "Prints the current user and its groups" }
func (w *WhoamiCommand) Usage() string            { return "whoami\n" }
func (w *WhoamiCommand) SetFlags(f *flag.FlagSet) {}

func (w *WhoamiCommand) Execute(ctx context.Context, _ *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	var credSource string
	if len(args) > 0 {
		if v, ok := args[0].(string); ok {
			credSource = v
		}
	}
	loader, err := mtls.Loader(credSource)
	if err != nil {
		fmt.Printf("failed to load %s certificate loader: %v\n", credSource, err)
		return subcommands.ExitFailure
	}

	clientCertInfo, err := loader.GetClientCertInfo(ctx)
	if err != nil {
		fmt.Printf("failed to get client cert info: %v\n", err)
		return subcommands.ExitFailure
	}
	showClientInfoFromClientCertInfo(*clientCertInfo)
	return subcommands.ExitSuccess
}
