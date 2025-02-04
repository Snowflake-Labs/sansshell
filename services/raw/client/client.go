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

// Package client provides subcommands that use proto reflection to
// call other services built into sansshell.
package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/subcommands"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/Snowflake-Labs/sansshell/client"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "raw"

func init() {
	subcommands.Register(&rawCmd{}, subPackage)
}

func (*rawCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&callCmd{}, "")
	return c
}

type rawCmd struct{}

func (*rawCmd) Name() string { return subPackage }
func (p *rawCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *rawCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*rawCmd) SetFlags(f *flag.FlagSet) {}

func (p *rawCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

type repeatedString []string

func (i *repeatedString) String() string {
	if i == nil {
		return "[]"
	}
	return fmt.Sprint([]string(*i))
}

func (i *repeatedString) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type callCmd struct {
	metadata repeatedString
}

func (*callCmd) Name() string     { return "call" }
func (*callCmd) Synopsis() string { return "Call sansshell-server with a hand-written RPC request" }
func (*callCmd) Usage() string {
	return `call <method> [json-encoded-request]:
  Call sansshell-server with a hand-written RPC request. The request can be provided either
  as an argument to the command or through stdin. The method being called must be from a
  proto definition compiled into sanssh.

  Examples:
    sanssh raw call /HealthCheck.HealthCheck/Ok '{}'
	echo '{"command":"/bin/echo", "args":["Hello World"]}' | sanssh raw call /Exec.Exec/Run

`
}

func (p *callCmd) SetFlags(f *flag.FlagSet) {
	f.Var(&p.metadata, "metadata", "key=value pair for setting in GRPC metadata separated, may be specified multiple times.")
}

func (p *callCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	for _, m := range p.metadata {
		pair := strings.SplitN(m, "=", 2)
		if len(pair) != 2 {
			fmt.Fprintf(os.Stderr, "metadata %v is missing equals sign\n", m)
			return subcommands.ExitUsageError

		}
		ctx = metadata.AppendToOutgoingContext(ctx, pair[0], pair[1])
	}

	var input io.Reader
	switch f.NArg() {
	case 0:
		fmt.Fprintln(os.Stderr, "Please specify a method name, like /HealthCheck.HealthCheck/Ok.")
		return subcommands.ExitUsageError
	case 1:
		input = os.Stdin
	case 2:
		input = strings.NewReader(f.Arg(1))
	default:
		fmt.Fprintln(os.Stderr, "Too many args provided, should be <method name> <request>")
		return subcommands.ExitUsageError
	}
	methodName := f.Arg(0)

	err := SendRequest(ctx, methodName, input, state.Conn, func(resp *ProxyResponse) error {
		if resp.Error == io.EOF {
			// Streaming commands may return EOF
			return nil
		}
		if resp.Error != nil {
			fmt.Fprintf(state.Err[resp.Index], "%v\n", resp.Error)
			return nil
		}
		prettyPrinted := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Format(resp.Resp)
		fmt.Fprintln(state.Out[resp.Index], prettyPrinted)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
