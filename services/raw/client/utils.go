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
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type ErrFailedToDecodeInput struct {
	Err error
}

func (e *ErrFailedToDecodeInput) Error() string {
	return fmt.Sprintf("failed to decode input: %v", e.Err)
}

// decodeExactlyOne decodes a single json message from the decoder and
// fails if more messages are present.
func decodeExactlyOne(decoder *json.Decoder, msg proto.Message) error {
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return &ErrFailedToDecodeInput{Err: fmt.Errorf("unable to read input message: %v", err)}
	}
	if err := protojson.Unmarshal([]byte(raw), msg); err != nil {
		return &ErrFailedToDecodeInput{Err: fmt.Errorf("unable to parse input json: %v", err)}
	}
	if decoder.More() {
		return &ErrFailedToDecodeInput{Err: fmt.Errorf("more than one input object provided for non-streaming call")}
	}
	return nil
}

func processResponse(resp *ProxyResponse, responseProcessors []ResponseProcessor) error {
	for _, processor := range responseProcessors {
		if err := processor(resp); err != nil {
			return err
		}
	}
	return nil
}

func processStreamedResponse(stream StreamingClientProxy, responseProcessors []ResponseProcessor) error {
	for {
		rs, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		for _, r := range rs {
			if err := processResponse(r, responseProcessors); err != nil {
				return err
			}
		}
	}
}

type ResponseProcessor func(*ProxyResponse) error

type ErrUnknownMethod struct {
	MethodName   string
	KnownMethods []string
}

func (e *ErrUnknownMethod) Error() string {
	return fmt.Sprintf("Unknown method %q, known ones are %v\n", e.MethodName, e.KnownMethods)
}

type ErrUnknownMessage struct {
	MessageDescriptor protoreflect.MessageDescriptor
	err               error
}

func (e *ErrUnknownMessage) Error() string {
	return fmt.Sprintf("Unable to find %v in protoregistry: %v\n", e.MessageDescriptor.FullName(), e.err)
}

// InvokeMethod sends a request with the specified method name and payload data to the given proxy connection, then
// processes the response using the provided ResponseProcessor functions.
// It supports both unary and streaming RPC calls.
// The method name must be in the format "/PackageName.ServiceName/MethodName".
// The payload should be a JSON-encoded request message. It can be provided as an io.Reader, allowing for streaming input.
func InvokeMethod(ctx context.Context, methodName string, payload io.Reader, conn *proxy.Conn, responseProcessors ...ResponseProcessor) error {
	inputDecoder := json.NewDecoder(payload)
	proxy := GenericClientProxy{conn: conn}
	// Find our method
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
		return &ErrUnknownMethod{MethodName: methodName, KnownMethods: allMethodNames}
	}

	// Figure out the proto types to use for requests and responses.
	inType, err := protoregistry.GlobalTypes.FindMessageByName(methodDescriptor.Input().FullName())
	if err != nil {
		return &ErrUnknownMessage{MessageDescriptor: methodDescriptor.Input(), err: err}
	}
	outType, err := protoregistry.GlobalTypes.FindMessageByName(methodDescriptor.Output().FullName())
	if err != nil {
		return &ErrUnknownMessage{MessageDescriptor: methodDescriptor.Output(), err: err}
	}

	// Make our actual call, using different ways based on the streaming options.
	if methodDescriptor.IsStreamingClient() {
		stream, err := proxy.StreamingOneMany(ctx, methodName, methodDescriptor, outType)
		if err != nil {
			return fmt.Errorf("%v\n", err)
		}

		var g errgroup.Group

		g.Go(func() error {
			for inputDecoder.More() {
				var raw json.RawMessage
				if err := inputDecoder.Decode(&raw); err != nil {
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
		g.Go(func() error { return processStreamedResponse(stream, responseProcessors) })
		if err := g.Wait(); err != nil {
			return fmt.Errorf("%v\n", err)
		}
	} else if !methodDescriptor.IsStreamingClient() && methodDescriptor.IsStreamingServer() {
		in := inType.New().Interface()
		if err := decodeExactlyOne(inputDecoder, in); err != nil {
			return fmt.Errorf("%v\n", err)
		}

		stream, err := proxy.StreamingOneMany(ctx, methodName, methodDescriptor, outType)
		if err != nil {
			return fmt.Errorf("%v\n", err)
		}
		if err := stream.SendMsg(in); err != nil {
			return fmt.Errorf("%v\n", err)
		}
		if err := stream.CloseSend(); err != nil {
			return fmt.Errorf("%v\n", err)
		}
		if err := processStreamedResponse(stream, responseProcessors); err != nil {
			return fmt.Errorf("%v\n", err)
		}
	} else {
		// It's a unary call if neither client or server has streams.
		in := inType.New().Interface()
		if err := decodeExactlyOne(inputDecoder, in); err != nil {
			return fmt.Errorf("%v\n", err)
		}
		resp, err := proxy.UnaryOneMany(ctx, methodName, in, outType)
		if err != nil {
			return fmt.Errorf("%v\n", err)
		}
		for r := range resp {
			err := processResponse(r, responseProcessors)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
