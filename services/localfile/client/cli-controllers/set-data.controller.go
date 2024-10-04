/* Copyright (c) 2024 Snowflake Inc. All rights reserved.

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

package cli_controllers

import (
	"context"
	"errors"
	"flag"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	cliUtils "github.com/Snowflake-Labs/sansshell/services/util/cli"
	"github.com/google/subcommands"
	"os"
	"strings"
)

// setDataCmd cli adapter for execution infrastructure implementation of [subcommands.Command] interface
type setDataCmd struct {
	fileFormat pb.FileFormat
	valueType  pb.DataSetValueType
	cliLogger  cliUtils.StyledCliLogger
}

func (*setDataCmd) Name() string { return "set-data" }
func (*setDataCmd) Synopsis() string {
	return "Get data from file of specific format. Currently supported: yml, dotenv"
}
func (*setDataCmd) Usage() string {
	return `sanssh <sanssh-args> file set-data [--format <file-format>] [--value-type <value-type>] <file-path> <data-key> <value>
  Where:
    - <sanssh-args> common sanssh arguments
    - <file-path> is the path to the file on remote machine. If --format is not provided, format would be detected from file extension.
    - <data-key> is the key to set value in the file. For different file formats it would require keys in different format
      - for "yml", key should be valid YAMLPath string
      - for "dotenv", key should be a name of variable
    - <value> is the value to set in the file

  Flags:
`
}

func (p *setDataCmd) SetFlags(f *flag.FlagSet) {
	f.Func(
		"value-type",
		`type of value to set in the file. Supported types are:
  - string (default)
  - int
  - float
  - bool
`,
		func(s string) error {
			lowerCased := strings.ToLower(s)
			switch lowerCased {
			case "string":
				p.valueType = pb.DataSetValueType_STRING_VAL
			case "int":
				p.valueType = pb.DataSetValueType_INT_VAL
			case "float":
				p.valueType = pb.DataSetValueType_FLOAT_VAL
			case "bool":
				p.valueType = pb.DataSetValueType_BOOL_VAL
			default:
				return errors.New("invalid value type")
			}

			return nil
		})

	f.Func("format", "File format (Optional). Could be one of: yml, dotenv", func(s string) error {
		lowerCased := strings.ToLower(s)

		switch lowerCased {
		case "yml":
			p.fileFormat = pb.FileFormat_YML
		case "dotenv":
			p.fileFormat = pb.FileFormat_DOTENV
		default:
			return errors.New("invalid file format")
		}

		return nil
	})
}

// Execute is a method handle command execution. It adapter between cli and business logic
func (p *setDataCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)

	if len(f.Args()) < 1 {
		p.cliLogger.Errorc(cliUtils.RedText, "File path is missing.\n")
		return subcommands.ExitUsageError
	}

	if len(f.Args()) < 2 {
		p.cliLogger.Errorc(cliUtils.RedText, "Data path is missing.\n")
		return subcommands.ExitUsageError
	}

	if len(f.Args()) < 3 {
		p.cliLogger.Errorc(cliUtils.RedText, "Data value is missing.\n")
		return subcommands.ExitUsageError
	}

	remoteFilePath := f.Arg(0)
	dataKey := f.Arg(1)
	dataValue := f.Arg(2)
	dataValueType := p.valueType

	fileFormat := p.fileFormat
	if fileFormat == pb.FileFormat_UNKNOWN {
		fileFormatFromExt, err := getFileTypeFromPath(remoteFilePath)
		if err != nil {
			p.cliLogger.Errorfc(cliUtils.RedText, "Could not set data in filepath: %s\n", err.Error())
			return subcommands.ExitUsageError
		}

		fileFormat = fileFormatFromExt
	}

	preloader := cliUtils.NewDotPreloader("Waiting for results from remote machines", util.IsStreamToTerminal(os.Stdout))
	client := pb.NewLocalFileClientProxy(state.Conn)

	p.cliLogger.Infof("Setting data in file %s\n", dataValueType)
	preloader.Start()

	responses, err := client.DataSetOneMany(ctx, &pb.DataSetRequest{
		Filename:   remoteFilePath,
		DataKey:    dataKey,
		FileFormat: fileFormat,
		Value:      dataValue,
		ValueType:  dataValueType,
	})

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

		status := cliUtils.CGreen("Ok")
		if resp.Error != nil {
			status = cliUtils.Colorizef(cliUtils.RedText, "Failed due to error - %s", resp.Error.Error())
		}

		targetLogger.Infof("Value set status: %s\n", status)
		preloader.Start()
	}
	preloader.StopWith("Completed.\n")

	return subcommands.ExitSuccess
}

func NewDataSetCmd() subcommands.Command {
	return &setDataCmd{
		fileFormat: pb.FileFormat_UNKNOWN,
		valueType:  pb.DataSetValueType_STRING_VAL,
		cliLogger: cliUtils.NewStyledCliLogger(os.Stdout, os.Stderr, &cliUtils.CliLoggerOptions{
			ApplyStylingForErr: util.IsStreamToTerminal(os.Stderr),
			ApplyStylingForOut: util.IsStreamToTerminal(os.Stdout),
		}),
	}
}
