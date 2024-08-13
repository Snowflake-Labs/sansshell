/* Copyright (c) 2022 Snowflake Inc. All rights reserved.

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

package input

import (
	"context"
	"flag"
	pb "github.com/Snowflake-Labs/sansshell/services/network"
	app "github.com/Snowflake-Labs/sansshell/services/network/client/application"
	"github.com/Snowflake-Labs/sansshell/services/util"
	cliUtils "github.com/Snowflake-Labs/sansshell/services/util/cli"
	"github.com/Snowflake-Labs/sansshell/services/util/validator"
	"github.com/google/subcommands"
	"os"
)

// TCPCheckCmd cli adapter for execution infrastructure implementation of [subcommands.Command] interface
type TCPCheckCmd struct {
	host      string
	port      int
	timeout   uint
	cliLogger cliUtils.StyledCliLogger
}

func (*TCPCheckCmd) Name() string { return "tcp-check" }
func (*TCPCheckCmd) Synopsis() string {
	return "Check tcp connectivity from remote machine to specified server"
}
func (*TCPCheckCmd) Usage() string {
	return `tcp-check --host <host> --port <port>:
    Makes tcp connectivity check from the remote machine to specified server.
`
}

func (p *TCPCheckCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.host, "host", "", "Host to check connectivity from remote machine")
	f.IntVar(&p.port, "port", 8080, "Port to check connectivity from remote machine")
	f.UintVar(&p.timeout, "timeout", 3, "Timeout in seconds to wait for response from --host on remote machine")
}

// Execute is a method handle command execution. It adapter between cli and business logic
func (p *TCPCheckCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)

	if p.host == "" {
		p.cliLogger.Errorc(cliUtils.RedText, "--host flag is required.\n")
		return subcommands.ExitUsageError
	}

	if err := validator.IsValidPort(p.port); err != nil {
		p.cliLogger.Errorfc(cliUtils.RedText, "Invalid port number: %s\n", err.Error())
		return subcommands.ExitUsageError
	}

	preloader := cliUtils.NewDotPreloader("Waiting for results from remote machines")
	client := pb.NewNetworkClientProxy(state.Conn)
	usecase := app.NewTCPCheckUseCase(client)

	preloader.Start()
	results, err := usecase.Run(ctx, p.host, uint8(p.port), p.timeout)
	if err != nil {
		preloader.Stop()
		p.cliLogger.Errorfc(cliUtils.RedText, "Unexpected error: %s\n", err.Error())
		return subcommands.ExitFailure
	}

	for result := range results {
		preloader.Stop()
		targetLogger := cliUtils.NewStyledCliLogger(state.Out[result.Index], state.Err[result.Index])

		var status cliUtils.StyledText
		if result.Error != nil {
			status = cliUtils.Colorizef(cliUtils.RedText, "Unexpected error - %s", result.Error.Error())
		} else if result.Resp.Ok {
			status = cliUtils.Colorize(cliUtils.GreenText, "Succeed")
		} else {
			status = cliUtils.Colorizef(cliUtils.RedText, "Failed - %s", *result.Resp.FailReason)
		}

		targetLogger.Infof(
			"Test %s, status: %s\n",
			cliUtils.Colorizef(cliUtils.YellowText, "%s:%d", p.host, p.port),
			status,
		)

		preloader.Start()
	}
	preloader.StopWith("Connectivity check completed.\n")

	return subcommands.ExitSuccess
}

func NewTCPCheckCmd() *TCPCheckCmd {
	return &TCPCheckCmd{
		cliLogger: cliUtils.NewStyledCliLogger(os.Stdout, os.Stderr),
	}
}
