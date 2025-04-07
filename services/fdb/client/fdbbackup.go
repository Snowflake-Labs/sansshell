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
	"os"
	"strings"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	execpb "github.com/Snowflake-Labs/sansshell/services/exec"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const fdbBackupPackage = "fdbbackup"

func init() {
	subcommands.Register(&fdbBackupCmd{}, "")
}

type fdbBackupCmd struct{}

func (*fdbBackupCmd) Name() string             { return fdbBackupPackage }
func (*fdbBackupCmd) SetFlags(_ *flag.FlagSet) {}

func (p *fdbBackupCmd) Synopsis() string {
	return "Run fdbbackup command on the given host"
}

func (p *fdbBackupCmd) Usage() string {
	return `fdbbackup <subcommand> [flags]:
  Run fdbbackup with the given subcommand.

  Supported subcommands:
  start        - Start a backup for the given destination
  status       - Get the status of a backup
  abort        - Abort a backup
  describe     - Describe a backup
  delete       - Delete a backup
  tags         - List all backup tags
  help         - Display help information for fdbbackup
`
}

func (p *fdbBackupCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := &util.ExecuteState{
		Conn: args[0].(*proxy.Conn),
	}

	if f.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "fdbbackup requires a subcommand")
		return subcommands.ExitUsageError
	}

	subcommand := f.Arg(0)
	validSubcommands := []string{"start", "status", "abort", "describe", "delete", "tags", "help"}
	
	isValid := false
	for _, cmd := range validSubcommands {
		if subcommand == cmd {
			isValid = true
			break
		}
	}
	
	if !isValid {
		fmt.Fprintf(os.Stderr, "Invalid subcommand: %s. Valid subcommands are: %s\n", 
			subcommand, strings.Join(validSubcommands, ", "))
		return subcommands.ExitUsageError
	}

	// Build the command arguments
	cmdArgs := []string{subcommand}
	
	// Add any additional arguments
	for i := 1; i < f.NArg(); i++ {
		cmdArgs = append(cmdArgs, f.Arg(i))
	}

	// Add all flags passed to the subcommand
	f.Visit(func(flg *flag.Flag) {
		// For boolean flags, add just the flag if true
		if flg.Value.String() == "true" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("-%s", flg.Name))
		} else if flg.Value.String() != "false" && flg.Value.String() != "" {
			// For non-boolean flags with values, add flag and value
			cmdArgs = append(cmdArgs, fmt.Sprintf("-%s", flg.Name), flg.Value.String())
		}
	})

	clients := execpb.NewExecClientProxy(state.Conn)
	
	req := &execpb.ExecRequest{
		Command: "/usr/local/bin/fdbbackup",
		Args:    cmdArgs,
	}

	respChan, err := clients.RunOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return subcommands.ExitFailure
	}

	status := subcommands.ExitSuccess
	for resp := range respChan {
		if resp.Error != nil {
			fmt.Fprintf(os.Stderr, "Error from %s: %v\n", resp.Target, resp.Error)
			status = subcommands.ExitFailure
			continue
		}

		fmt.Printf("=== Output from %s (exit code %d) ===\n", resp.Target, resp.Resp.RetCode)
		fmt.Println(string(resp.Resp.Stdout))

		if len(resp.Resp.Stderr) > 0 {
			fmt.Fprintf(os.Stderr, "=== Error output from %s ===\n", resp.Target)
			fmt.Fprintln(os.Stderr, string(resp.Resp.Stderr))
		}

		if resp.Resp.RetCode != 0 {
			status = subcommands.ExitFailure
		}
	}

	return status
}