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

package client

import (
	"context"
	"fmt"
	"io"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GenericClientProxy is similar to generated sansshell proxy code but works with generic protobufs.
// This code is largely copied from the generated grpcproxy code.
type GenericClientProxy struct {
	conn *proxy.Conn
}

// ProxyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type ProxyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  proto.Message
	Error error
}

// UnaryOneMany sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *GenericClientProxy) UnaryOneMany(ctx context.Context, method string, in proto.Message, out protoreflect.MessageType, opts ...grpc.CallOption) (<-chan *ProxyResponse, error) {
	conn := c.conn
	ret := make(chan *ProxyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ProxyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   out.New().Interface(),
			}
			err := conn.Invoke(ctx, method, in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, method, in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrieve untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ProxyResponse{
				Resp: out.New().Interface(),
			}

			resp, ok := <-manyRet
			if !ok {
				// All done so we can shut down.
				close(ret)
				return
			}
			typedResp.Target = resp.Target
			typedResp.Index = resp.Index
			typedResp.Error = resp.Error
			if resp.Error == nil {
				if err := resp.Resp.UnmarshalTo(typedResp.Resp); err != nil {
					typedResp.Error = fmt.Errorf("can't decode any response - %v. Original Error - %v", err, resp.Error)
				}
			}
			ret <- typedResp
		}
	}()

	return ret, nil
}

type StreamingClientProxy interface {
	Recv() ([]*ProxyResponse, error)
	grpc.ClientStream
}

type ClientStreamingClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	outType    protoreflect.MessageType
	grpc.ClientStream
}

func (x *ClientStreamingClientProxy) Recv() ([]*ProxyResponse, error) {
	var ret []*ProxyResponse
	// If this is a direct connection the RecvMsg call is to a standard grpc.ClientStream
	// and not our proxy based one. This means we need to receive a typed response and
	// convert it into a single slice entry return. This ensures the OneMany style calls
	// can be used by proxy with 1:N targets and non proxy with 1 target without client changes.
	if x.cc.Direct() {
		// Check if we're done. Just return EOF now. Any real error was already sent inside
		// of a ManyResponse.
		if x.directDone {
			return nil, io.EOF
		}
		m := x.outType.New().Interface()
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &ProxyResponse{
			Resp:   m,
			Error:  err,
			Target: x.cc.Targets[0],
			Index:  0,
		})
		// An error means we're done so set things so a later call now gets an EOF.
		if err != nil {
			x.directDone = true
		}
		return ret, nil
	}

	m := []*proxy.Ret{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	for _, r := range m {
		typedResp := &ProxyResponse{
			Resp: x.outType.New().Interface(),
		}
		typedResp.Target = r.Target
		typedResp.Index = r.Index
		typedResp.Error = r.Error
		if r.Error == nil {
			if err := r.Resp.UnmarshalTo(typedResp.Resp); err != nil {
				typedResp.Error = fmt.Errorf("can't decode any response - %v. Original Error - %v", err, r.Error)
			}
		}
		ret = append(ret, typedResp)
	}
	return ret, nil
}

// StreamingOneMany sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *GenericClientProxy) StreamingOneMany(ctx context.Context, method string, methodDescriptor protoreflect.MethodDescriptor, out protoreflect.MessageType, opts ...grpc.CallOption) (StreamingClientProxy, error) {
	streamDesc := &grpc.StreamDesc{
		StreamName:    string(methodDescriptor.Name()),
		ClientStreams: methodDescriptor.IsStreamingClient(),
		ServerStreams: methodDescriptor.IsStreamingServer(),
	}
	stream, err := c.conn.NewStream(ctx, streamDesc, method, opts...)
	if err != nil {
		return nil, err
	}
	x := &ClientStreamingClientProxy{c.conn, false, out, stream}
	return x, nil
}
