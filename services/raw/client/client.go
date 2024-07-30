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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/google/subcommands"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

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

// decodeExactlyOne decodes a single json message from the decoder and
// fails if more messages are present.
func decodeExactlyOne(decoder *json.Decoder, msg proto.Message) error {
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("unable to read input message: %v", err)
	}
	if err := protojson.Unmarshal([]byte(raw), msg); err != nil {
		return fmt.Errorf("unable to parse input json: %v", err)
	}
	if decoder.More() {
		return fmt.Errorf("more than one input object provided for non-streaming call")
	}
	return nil
}

func printOutput(state *util.ExecuteState, resp *proxyResponse) {
	if resp.Error == io.EOF {
		// Streaming commands may return EOF
		return
	}
	if resp.Error != nil {
		fmt.Fprintf(state.Err[resp.Index], "%v\n", resp.Error)
		return
	}
	prettyPrinted := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Format(resp.Resp)
	fmt.Fprintln(state.Out[resp.Index], prettyPrinted)
}

func printAllFromStream(state *util.ExecuteState, stream streamingClientProxy) error {
	for {
		rs, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		for _, r := range rs {
			printOutput(state, r)
		}
	}

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
	proxy := genericClientProxy{state.Conn}

	for _, m := range p.metadata {
		pair := strings.SplitN(m, "=", 2)
		if len(pair) != 2 {
			fmt.Fprintf(os.Stderr, "metadata %v is missing equals sign\n", m)
			return subcommands.ExitUsageError

		}
		ctx = metadata.AppendToOutgoingContext(ctx, pair[0], pair[1])
	}

	var input *json.Decoder
	switch f.NArg() {
	case 0:
		fmt.Fprintln(os.Stderr, "Please specify a method name, like /HealthCheck.HealthCheck/Ok.")
		return subcommands.ExitUsageError
	case 1:
		input = json.NewDecoder(os.Stdin)
	case 2:
		input = json.NewDecoder(strings.NewReader(f.Arg(1)))
	default:
		fmt.Fprintln(os.Stderr, "Too many args provided, should be <method name> <request>")
		return subcommands.ExitUsageError
	}

	// Find our method
	methodName := f.Arg(0)
	var methodDescriptor protoreflect.MethodDescriptor
	var allMethodNames []string
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for i := 0; i < methods.Len(); i++ {
				method := methods.Get(i)
				fullName := fmt.Sprintf("/%s/%s", svc.FullName(), method.Name())
				if fullName == methodName {
					// We found the right one, no need to continue
					methodDescriptor = method
					return false
				}
				allMethodNames = append(allMethodNames, fullName)
			}
		}
		return true
	})
	if methodDescriptor == nil {
		sort.Strings(allMethodNames)
		fmt.Fprintf(os.Stderr, "Unknown method %q, known ones are %v\n", methodName, allMethodNames)
		return subcommands.ExitUsageError
	}

	// Figure out the proto types to use for requests and responses.
	inType, err := protoregistry.GlobalTypes.FindMessageByName(methodDescriptor.Input().FullName())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find %v in protoregistry: %v\n", methodDescriptor.Input().FullName(), err)
		return subcommands.ExitFailure
	}
	outType, err := protoregistry.GlobalTypes.FindMessageByName(methodDescriptor.Output().FullName())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find %v in protoregistry: %v\n", methodDescriptor.Output().FullName(), err)
		return subcommands.ExitFailure
	}

	// Make our actual call, using different ways based on the streaming options.
	if methodDescriptor.IsStreamingClient() {
		stream, err := proxy.StreamingOneMany(ctx, methodName, methodDescriptor, outType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitFailure
		}

		var g errgroup.Group

		g.Go(func() error {
			for input.More() {
				var raw json.RawMessage
				if err := input.Decode(&raw); err != nil {
					return fmt.Errorf("unable to read input message: %v", err)
				}
				msg := inType.New().Interface()
				if err := protojson.Unmarshal([]byte(raw), msg); err != nil {
					return fmt.Errorf("unable to parse input json: %v", err)
				}
				if err := stream.SendMsg(msg); err != nil {
					return err
				}
			}
			return stream.CloseSend()
		})
		g.Go(func() error { return printAllFromStream(state, stream) })
		if err := g.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitFailure
		}
	} else if !methodDescriptor.IsStreamingClient() && methodDescriptor.IsStreamingServer() {
		in := inType.New().Interface()
		if err := decodeExactlyOne(input, in); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitUsageError
		}

		stream, err := proxy.StreamingOneMany(ctx, methodName, methodDescriptor, outType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitFailure
		}
		if err := stream.SendMsg(in); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitFailure
		}
		if err := stream.CloseSend(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitFailure
		}
		if err := printAllFromStream(state, stream); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitFailure
		}
	} else {
		// It's a unary call if neither client or server has streams.
		in := inType.New().Interface()
		if err := decodeExactlyOne(input, in); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitUsageError
		}
		resp, err := proxy.UnaryOneMany(ctx, methodName, in, outType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return subcommands.ExitFailure
		}
		for r := range resp {
			printOutput(state, r)
		}

	}
	return subcommands.ExitSuccess
}
