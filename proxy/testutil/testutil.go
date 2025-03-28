/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

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

// Package testutil contains helpers and utilities for
// writing unittests against the sansshell proxy.
package testutil

import (
	"context"
	"errors"
	"fmt"
	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"io"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	pb "github.com/Snowflake-Labs/sansshell/proxy"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

// Exchange is a test helper for the common pattern of trading messages with
// a proxy server over an open stream.
// Errors encountered during send/receive will cause `t` to fail.
// `req` may be nil, in which case this only performs a Recv
func Exchange(t *testing.T, stream pb.Proxy_ProxyClient, req *pb.ProxyRequest) *pb.ProxyReply {
	t.Helper()
	if req != nil {
		err := stream.Send(req)
		testutil.FatalOnErr(fmt.Sprintf("ProxyClient.Send(%v)", req), err, t)
	}
	reply, err := stream.Recv()
	testutil.FatalOnErr("ProxyClient.Recv()", err, t)
	return reply
}

// StartStream establishes a new target stream through the proxy connection in `stream`.
// Will fail `t` on any errors communicating with the proxy, or if the returned response from
// the proxy is not a valid StartStreamReply.
func StartStream(t *testing.T, stream pb.Proxy_ProxyClient, target, method string, dialTimeout ...time.Duration) *pb.StartStreamReply {
	t.Helper()
	nonce := rand.Uint32()
	var dt *durationpb.Duration
	if len(dialTimeout) != 0 {
		dt = durationpb.New(dialTimeout[0])
	}
	req := &pb.ProxyRequest{
		Request: &pb.ProxyRequest_StartStream{
			StartStream: &pb.StartStream{
				Target:      target,
				MethodName:  method,
				Nonce:       nonce,
				DialTimeout: dt,
			},
		},
	}
	reply := Exchange(t, stream, req)
	switch reply.Reply.(type) {
	case *pb.ProxyReply_StartStreamReply:
		ssr := reply.GetStartStreamReply()
		if ssr.Nonce != nonce {
			t.Fatalf("StartStream(%s, %s) mismatched nonce, want %d, got %d", target, method, nonce, ssr.Nonce)
		}
		return ssr
	default:
		t.Fatalf("StartStream(%s, %s) got reply of type %T, want StartStreamReply", target, method, reply.Reply)
	}
	return nil
}

// MustStartStream invokes StartStream, but fails `t` if the response does not contain a valid
// stream id. Returns the created stream id.
func MustStartStream(t *testing.T, stream pb.Proxy_ProxyClient, target, method string, dialTimeout ...time.Duration) uint64 {
	t.Helper()
	reply := StartStream(t, stream, target, method, dialTimeout...)
	if reply.GetStreamId() == 0 {
		t.Fatalf("MustStartStream(%s, %s) want response with valid stream ID, got %+v", target, method, reply)
	}
	return reply.GetStreamId()
}

// PackStreamData creates a StreamData request for the supplied streamIds, with `req` as
// the payload.
// Any error in creation will fail `t`
func PackStreamData(t *testing.T, req proto.Message, streamIds ...uint64) *pb.ProxyRequest {
	t.Helper()
	packed, err := anypb.New(req)
	testutil.FatalOnErr(fmt.Sprintf("anypb.New(%+v)", req), err, t)
	return &pb.ProxyRequest{
		Request: &pb.ProxyRequest_StreamData{
			StreamData: &pb.StreamData{
				StreamIds: streamIds,
				Payload:   packed,
			},
		},
	}
}

// UnpackStreamData will unmarshal a StreamData entry into a slice of stream ids
// and the message associated with it.
func UnpackStreamData(t *testing.T, reply *pb.ProxyReply) ([]uint64, proto.Message) {
	t.Helper()
	sd := reply.GetStreamData()
	if sd == nil {
		t.Fatalf("UnpackStreamData() reply was of type %T, want StreamData", reply.Reply)
	}
	data, err := sd.Payload.UnmarshalNew()
	testutil.FatalOnErr(fmt.Sprintf("anypb.UnmarshalNew(%v)", sd), err, t)
	return sd.StreamIds, data
}

// EchoTestDataServer is a TestDataServiceServer for testing
type EchoTestDataServer struct {
	serverName string
}

// TestUnary implements the service for EchoTestDataServer
func (e *EchoTestDataServer) TestUnary(ctx context.Context, req *tdpb.TestRequest) (*tdpb.TestResponse, error) {
	if req.Input == "error" {
		return nil, errors.New("error")
	}
	return &tdpb.TestResponse{
		Output: fmt.Sprintf("%s %s", e.serverName, req.Input),
	}, nil
}

// TestServerStream implements the service for EchoTestDataServer
func (e *EchoTestDataServer) TestServerStream(req *tdpb.TestRequest, stream tdpb.TestService_TestServerStreamServer) error {
	if req.Input == "error" {
		return errors.New("error")
	}
	for i := 0; i < 5; i++ {
		if err := stream.Send(&tdpb.TestResponse{
			Output: fmt.Sprintf("%s %d %s", e.serverName, i, req.Input),
		}); err != nil {
			return err
		}
	}
	return nil
}

// TestClientStream implements the service for EchoTestDataServer
func (e *EchoTestDataServer) TestClientStream(stream tdpb.TestService_TestClientStreamServer) error {
	var inputs []string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			for _, i := range inputs {
				if i == "error" {
					return errors.New("error")
				}
			}
			return stream.SendAndClose(&tdpb.TestResponse{
				Output: fmt.Sprintf("%s %s", e.serverName, strings.Join(inputs, ",")),
			})
		}
		if err != nil {
			return err
		}
		inputs = append(inputs, req.Input)
	}
}

// TestBidiStream implements the service for EchoTestDataServer
func (e *EchoTestDataServer) TestBidiStream(stream tdpb.TestService_TestBidiStreamServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if req.Input == "error" {
			return errors.New("error")
		}
		if err := stream.Send(&tdpb.TestResponse{
			Output: fmt.Sprintf("%s %s", e.serverName, req.Input),
		}); err != nil {
			return nil
		}
	}
}

// NewOpaRPCAuthorizer generates a new authorizer with the given policy. Will handle errors for testing.
func NewOpaRPCAuthorizer(ctx context.Context, t *testing.T, policy string) rpcauth.RPCAuthorizer {
	t.Helper()
	auth, err := opa.NewOpaRPCAuthorizer(ctx, policy)
	testutil.FatalOnErr(fmt.Sprintf("rpcauth.NewWithPolicy(%s)", policy), err, t)
	return auth
}

// NewAllowAllRPCAuthorizer generates a new authorizer which allows all RPCs to pass through.
func NewAllowAllRPCAuthorizer(ctx context.Context, t *testing.T) rpcauth.RPCAuthorizer {
	policy := `
package sansshell.authz
default allow = true
`
	return NewOpaRPCAuthorizer(ctx, t, policy)
}

// WithBufDialer returns a DialOption which will lookup a bufnet connection in the
// map passed to it. Allows arbitrary N backend servers to run simultaneously.
func WithBufDialer(m map[string]*bufconn.Listener) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
		l := m[target]
		if l == nil {
			return nil, fmt.Errorf("no conn for target %s", target)
		}
		c, err := l.Dial()
		if err != nil {
			return nil, err
		}
		return &targetConn{Conn: c, target: target}, nil
	})
}

// BufSize is the buffer size used for a bufnet listener.
const BufSize = 1024 * 1024

// targetAddr is a net.Addr that returns a target as it's address.
type targetAddr string

func (t targetAddr) String() string {
	return string(t)
}

func (t targetAddr) Network() string {
	return "bufconn"
}

// targetConn is a net.Conn that returns a targetAddr for its
// remote address
type targetConn struct {
	net.Conn
	target string
}

func (t *targetConn) RemoteAddr() net.Addr {
	return targetAddr(t.target)
}

// StartTestDataServer will start the given server running (as a separate Go routine)
// and return the Listener to connect to it over. The server will be automatically
// stopped when the enclosing test exits.
func StartTestDataServer(t *testing.T, serverName string) *bufconn.Listener {
	t.Helper()
	lis := bufconn.Listen(BufSize)
	echoServer := &EchoTestDataServer{serverName: serverName}
	rpcServer := grpc.NewServer()
	tdpb.RegisterTestServiceServer(rpcServer, echoServer)
	go func() {
		// Don't care about errors here as they might come on shutdown and we
		// can't log through t at that point anyways.
		_ = rpcServer.Serve(lis)
	}()
	t.Cleanup(func() {
		rpcServer.Stop()
	})
	return lis
}

// StartTestDataServers will start N servers using StartTestDataServer returning a map
// of server -> Listener
func StartTestDataServers(t *testing.T, serverNames ...string) map[string]*bufconn.Listener {
	t.Helper()
	out := map[string]*bufconn.Listener{}
	for _, name := range serverNames {
		out[name] = StartTestDataServer(t, name)
	}
	return out
}
