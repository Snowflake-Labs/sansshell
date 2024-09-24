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
	"errors"
	"flag"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	app "github.com/Snowflake-Labs/sansshell/services/localfile/client/application"
	"github.com/Snowflake-Labs/sansshell/services/util"
	cliUtils "github.com/Snowflake-Labs/sansshell/services/util/cli"
	"github.com/google/subcommands"
	"google.golang.org/grpc/status"
	"os"
	"strings"
)

// setDataCmd cli adapter for execution infrastructure implementation of [subcommands.Command] interface
type getDataCmd struct {
	fileFormat pb.FileFormat
	cliLogger  cliUtils.StyledCliLogger
}

func (*getDataCmd) Name() string { return "get-data" }
func (*getDataCmd) Synopsis() string {
	return "Get data from file of specific format. Currently supported: yml, dotenv"
}
func (*getDataCmd) Usage() string {
	return `get-data [--format=yml|dotenv] <file-path> <data-key>:
  Get value by 'data-key' from file of file by 'file-path' of specific format.
  Arguments:
    - file-path - path to file with data
    - data-key - key to read data from file. For different file format it should be:
        - yml - YmlPath string
        - dotenv - variable key

    Format could be detected from file extension or explicitly specified by --format flag.

  Flags:
`
}

func (p *getDataCmd) SetFlags(f *flag.FlagSet) {
	f.Func("format", "File format (Optional). Could be one of: yml, dotenv", func(s string) error {
		lowerCased := strings.ToLower(s)

		switch lowerCased {
		case "yml":
			p.fileFormat = pb.FileFormat_YML
		case "dotenv":
			p.fileFormat = pb.FileFormat_DOTENV
		default:
			return errors.New("could be only yml or dotenv")
		}

		return nil
	})
}

// Execute is a method handle command execution. It adapter between cli and business logic
func (p *getDataCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)

	if len(f.Args()) < 1 {
		p.cliLogger.Errorc(cliUtils.RedText, "File path is missing.\n")
		return subcommands.ExitUsageError
	}

	if len(f.Args()) < 2 {
		p.cliLogger.Errorc(cliUtils.RedText, "Property path is missing.\n")
		return subcommands.ExitUsageError
	}

	remoteFilePath := f.Arg(0)
	dataKey := f.Arg(1)

	fileFormat := p.fileFormat
	if fileFormat == pb.FileFormat_UNKNOWN {
		fileFormatFromExt, err := getFileTypeFromPath(remoteFilePath)
		if err != nil {
			p.cliLogger.Errorfc(cliUtils.RedText, "Could not get file type from filepath: %s\n", err.Error())
			return subcommands.ExitUsageError
		}

		fileFormat = fileFormatFromExt
	}

	preloader := cliUtils.NewDotPreloader("Waiting for results from remote machines", util.IsStreamToTerminal(os.Stdout))
	client := pb.NewLocalFileClientProxy(state.Conn)
	usecase := app.NewDataGetUsecase(client)

	preloader.Start()
	responses, err := usecase.Run(ctx, remoteFilePath, dataKey, fileFormat)

	if err != nil {
		preloader.Stop()
		p.cliLogger.Errorfc(cliUtils.RedText, "Unexpected error: %s\n", err.Error())
		return subcommands.ExitFailure
	}

	for resp := range responses {
		preloader.Stop()

		targetLogger := cliUtils.NewStyledCliLogger(state.Out[resp.Index], state.Err[resp.Index], &cliUtils.CliLoggerOptions{
			ApplyStylingForErr: util.IsStreamToTerminal(state.Err[resp.Index]),
			ApplyStylingForOut: util.IsStreamToTerminal(state.Out[resp.Index]),
		})

		if resp.Error != nil {
			st, _ := status.FromError(resp.Error)
			targetLogger.Errorfc(cliUtils.RedText,
				"Failed to get value: %s\n",
				st.Message(),
			)
			continue
		}
		targetLogger.Infof("Value: %s\n", resp.Resp.Value)

		preloader.Start()
	}
	preloader.StopWith("Completed.\n")

	return subcommands.ExitSuccess
}

func NewDataGetCmd() subcommands.Command {
	return &getDataCmd{
		fileFormat: pb.FileFormat_UNKNOWN,
		cliLogger: cliUtils.NewStyledCliLogger(os.Stdout, os.Stderr, &cliUtils.CliLoggerOptions{
			ApplyStylingForErr: util.IsStreamToTerminal(os.Stderr),
			ApplyStylingForOut: util.IsStreamToTerminal(os.Stdout),
		}),
	}
}
