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

package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauthz"
	pb "github.com/Snowflake-Labs/sansshell/proxy"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
	tu "github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func startTestProxyWithAuthz(ctx context.Context, t *testing.T, targets map[string]*bufconn.Listener, authz rpcauthz.RPCAuthorizer) pb.Proxy_ProxyClient {
	t.Helper()
	targetDialer := NewDialer(testutil.WithBufDialer(targets), grpc.WithTransportCredentials(insecure.NewCredentials()))
	lis := bufconn.Listen(testutil.BufSize)
	grpcServer := grpc.NewServer(grpc.StreamInterceptor(authz.AuthorizeStream))
	proxyServer := New(targetDialer, authz)
	proxyServer.Register(grpcServer)
	go func() {
		// Don't care about errors here as they might come on shutdown and we
		// can't log through t at that point anyways.
		_ = grpcServer.Serve(lis)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
	})
	bufMap := map[string]*bufconn.Listener{"proxy": lis}
	conn, err := grpc.DialContext(ctx, "proxy", testutil.WithBufDialer(bufMap), grpc.WithTransportCredentials(insecure.NewCredentials()))
	tu.FatalOnErr("DialContext(proxy)", err, t)
	stream, err := pb.NewProxyClient(conn).Proxy(ctx)
	tu.FatalOnErr("proxy.Proxy()", err, t)
	return stream
}

func startTestProxy(ctx context.Context, t *testing.T, targets map[string]*bufconn.Listener) pb.Proxy_ProxyClient {
	return startTestProxyWithAuthz(ctx, t, targets, testutil.NewAllowAllRPCAuthorizer(ctx, t))
}

func TestProxyServerStartStream(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:123")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	validMethods := []string{
		"/Testdata.TestService/TestUnary",
		"/Testdata.TestService/TestServerStream",
		"/Testdata.TestService/TestClientStream",
		"/Testdata.TestService/TestBidiStream",
	}

	// Test that valid methods yield streamIDs for reachable
	// hosts, and that the same host/method pair can be specified
	// multiple times, yielding unique stream ids
	for name := range testServerMap {
		for _, m := range validMethods {
			s1 := testutil.MustStartStream(t, proxyStream, name, m)
			s2 := testutil.MustStartStream(t, proxyStream, name, m)
			if s1 == s2 {
				t.Errorf("StartStream(%s, %s) duplicate id %d, want non-duplicate", name, m, s2)
			}
		}
	}

	// Now try a connection with a dialTimeout that shouldn't error but is smaller than 20s.
	for name := range testServerMap {
		testutil.MustStartStream(t, proxyStream, name, validMethods[0], 5*time.Second)
	}

	// Now try one with 0s which should error
	for name := range testServerMap {
		testutil.StartStream(t, proxyStream, name, validMethods[0], 0)
		preply, err := proxyStream.Recv()
		tu.FatalOnErr("ProxyClient.Recv()", err, t)
		t.Log(preply)
		if preply.GetServerClose() == nil {
			t.Fatal("didn't get ServerClose as expected for small dial")
		}
	}

	// Bad methods yield errors
	invalidMethods := []string{
		"bad format",
		"/nosuchpackage.nosuchservice/nosuchmethod",
		"/Testdata.TestService/NoSuchMethod",
	}

	// Starting streams for invalid methods yields an error
	for name := range testServerMap {
		for _, m := range invalidMethods {
			reply := testutil.StartStream(t, proxyStream, name, m)
			if c := reply.GetErrorStatus().GetCode(); codes.Code(c) != codes.InvalidArgument {
				t.Errorf("StartStream(%s, %s) reply was %v, want reply with code InvalidArgument", name, m, reply)
			}
		}
	}

	// Starting streams for invalid / unreahable hosts yields an error
	for _, name := range []string{"noSuchHost", "bogus"} {
		for _, m := range validMethods {
			reply := testutil.StartStream(t, proxyStream, name, m)
			if stat := reply.GetErrorStatus(); stat != nil {
				t.Errorf("startTargetStream(%s, %s) had error: %v", name, m, stat.Message)
				continue
			}
			preply, err := proxyStream.Recv()
			tu.FatalOnErr("ProxyClient.Recv()", err, t)

			if preply.GetServerClose() == nil {
				t.Errorf("startTargetStream(%s, %s) didn't get close", name, m)
				continue
			}
			if c := preply.GetServerClose().GetStatus().GetCode(); codes.Code(c) != codes.Unavailable {
				t.Errorf("startTargetStream(%s, %s) reply was %v(%d), want reply with code Unavailable", name, m, reply, c)
			}
		}
	}
}

func TestProxyServerCancel(t *testing.T) {
	// Test that streams can be cancelled
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	fooid := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")
	barid := testutil.MustStartStream(t, proxyStream, "bar:456", "/Testdata.TestService/TestUnary")

	cancelReq := &pb.ProxyRequest{
		Request: &pb.ProxyRequest_ClientCancel{
			ClientCancel: &pb.ClientCancel{
				StreamIds: []uint64{fooid, barid},
			},
		},
	}
	err := proxyStream.Send(cancelReq)
	tu.FatalOnErr("ClientCancel(foo, bar)", err, t)

	// we should receive two server close messages with status
	// cancelled
	for i := 0; i < 2; i++ {
		reply := testutil.Exchange(t, proxyStream, nil)
		switch reply.Reply.(type) {
		case *pb.ProxyReply_ServerClose:
			sc := reply.GetServerClose()
			if !strings.Contains(sc.GetStatus().GetMessage(), "canceled") {
				t.Errorf("ServerClose = %+v, want server close containing 'canceled", sc)
			}
		default:
			t.Fatalf("proxystream.Recv(), got reply of type %V, want ServerClose", reply.Reply)
		}
	}
}

func TestProxyServerUnaryCall(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")

	req := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "Foo"}, streamID)

	reply := testutil.Exchange(t, proxyStream, req)
	ids, data := testutil.UnpackStreamData(t, reply)
	if ids[0] != streamID {
		t.Errorf("StreamData.StreamIds[0] = %d, want %d", ids[0], streamID)
	}
	want := &tdpb.TestResponse{
		Output: "foo:123 Foo",
	}
	if !proto.Equal(data, want) {
		t.Errorf("proto.Equal(%v, %v) = false, want true", data, want)
	}

	reply = testutil.Exchange(t, proxyStream, nil)
	sc := reply.GetServerClose()
	if sc == nil {
		t.Fatalf("expected reply of type ServerClose, got %v", reply)
	}
	if sc.GetStatus() != nil {
		t.Errorf("ServerClose.Status, want nil, got %+v", sc.GetStatus())
	}
}

func TestProxyServerUnaryFanout(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	fooID := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")
	barID := testutil.MustStartStream(t, proxyStream, "bar:456", "/Testdata.TestService/TestUnary")

	req := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "Foo"}, fooID, barID)

	err := proxyStream.Send(req)
	tu.FatalOnErr(fmt.Sprintf("Send(%v)", req), err, t)

	// We send once request, but we'll get 4 replies:
	// one each of streamdata and server close for each
	// stream
	for i := 0; i < 4; i++ {
		reply := testutil.Exchange(t, proxyStream, nil)
		switch reply.Reply.(type) {
		case *pb.ProxyReply_ServerClose:
			sc := reply.GetServerClose()
			ids := sc.StreamIds
			switch ids[0] {
			case fooID:
				// expected
			case barID:
				// expected
			default:
				t.Fatalf("got response with id %d, want one of (%d, %d)", ids[0], fooID, barID)
			}
			if sc.GetStatus() != nil {
				t.Fatalf("got ServerClose with error %v, want nil", sc.GetStatus())
			}
		case *pb.ProxyReply_StreamData:
			ids, data := testutil.UnpackStreamData(t, reply)
			want := &tdpb.TestResponse{}
			switch ids[0] {
			case fooID:
				want.Output = "foo:123 Foo"
			case barID:
				want.Output = "bar:456 Foo"
			default:
				t.Fatalf("got response with id %d, want one of (%d, %d)", ids[0], fooID, barID)
			}
			if !proto.Equal(want, data) {
				t.Errorf("proto.Equal(%v, %v) for id %d was false, want true", ids[0], want, data)
			}
		default:
			t.Fatalf("unexpected reply type: %T", reply.Reply)
		}
	}
}

func TestProxyServerServerStream(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestServerStream")

	req := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "Foo"}, streamID)
	reply := testutil.Exchange(t, proxyStream, req)
	replies := []*pb.ProxyReply{reply}

	for i := 0; i < 4; i++ {
		replies = append(replies, testutil.Exchange(t, proxyStream, nil))
	}
	for i := 0; i < 5; i++ {
		want := &tdpb.TestResponse{
			Output: fmt.Sprintf("foo:456 %d Foo", i),
		}
		_, data := testutil.UnpackStreamData(t, replies[i])
		if !proto.Equal(want, data) {
			t.Errorf("Server response %d, proto.Equal(%v, %v) was false, want true", i, want, data)
		}
	}
}

func TestProxyServerServerStreamBadData(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestServerStream")

	req := testutil.PackStreamData(t, &emptypb.Empty{}, streamID)
	err := proxyStream.Send(req)
	tu.FatalOnErr("ProxyClient.Send", err, t)
	resp, err := proxyStream.Recv()
	tu.FatalOnErr(fmt.Sprintf("ProxyClient.Send(%v) no error for bad data", resp), err, t)
	if resp.GetServerClose() == nil {
		t.Fatalf("didn't get serverClose as expected. Got %T instead", resp.Reply)
	}

	t.Log(resp)
	// Close will return an error which is generally a canceled context.
	// A further read from the stream will show the reason since the stream itself
	// returns an error too (and you get at that via Recv)
	if resp.GetServerClose().GetStatus().Code == 0 {
		t.Fatal("didn't get non-zero status from Close as expected")
	}
	_, err = proxyStream.Recv()
	t.Log(err)
	if got, want, want2 := status.Code(err), codes.InvalidArgument, codes.Canceled; got != want && got != want2 {
		t.Fatalf("didn't get expected stream error code. got %s want %s or %s", codes.Code(got), codes.Code(want), codes.Code(want2))
	}
}

func TestProxyServerClientStream(t *testing.T) {
	ctx := context.Background()

	testServerMap := testutil.StartTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestClientStream")

	req := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "Foo"}, streamID)

	for i := 0; i < 3; i++ {
		err := proxyStream.Send(req)
		tu.FatalOnErr(fmt.Sprintf("%d: proxyStream.Send(%v)", i, req), err, t)
	}

	// Send a half-close, and exchange it for the reply
	hc := &pb.ProxyRequest{
		Request: &pb.ProxyRequest_ClientClose{
			ClientClose: &pb.ClientClose{
				StreamIds: []uint64{streamID},
			},
		},
	}

	want := &tdpb.TestResponse{
		Output: "foo:456 Foo,Foo,Foo",
	}
	reply := testutil.Exchange(t, proxyStream, hc)
	_, data := testutil.UnpackStreamData(t, reply)
	if !proto.Equal(want, data) {
		t.Errorf("client stream reply: proto.Equal(%v, %v) was false, want true", want, data)
	}

	// Final status
	reply = testutil.Exchange(t, proxyStream, nil)
	sc := reply.GetServerClose()
	if sc == nil {
		t.Fatalf("expected reply of type ServerClose, got %v", reply)
	}
	if sc.GetStatus() != nil {
		t.Errorf("ServerClose.Status, want nil, got %+v", sc.GetStatus())
	}
}

func TestProxyServerBidiStream(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestBidiStream")

	req := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "Foo"}, streamID)

	want := &tdpb.TestResponse{
		Output: "foo:456 Foo",
	}
	for i := 0; i < 10; i++ {
		reply := testutil.Exchange(t, proxyStream, req)
		_, data := testutil.UnpackStreamData(t, reply)
		if !proto.Equal(want, data) {
			t.Errorf("exchange %d, proto.Equal(%v, %v) was false, want true", i, want, data)
		}
	}

	// send a half-close, and wait for final stream status
	hc := &pb.ProxyRequest{
		Request: &pb.ProxyRequest_ClientClose{
			ClientClose: &pb.ClientClose{
				StreamIds: []uint64{streamID},
			},
		},
	}

	reply := testutil.Exchange(t, proxyStream, hc)

	sc := reply.GetServerClose()
	if sc == nil {
		t.Fatalf("expected reply of type ServerClose, got %v", reply)
	}
	if sc.GetStatus() != nil {
		t.Errorf("ServerClose.Status, want nil, got %+v", sc.GetStatus())
	}
}

func TestProxyServerProxyClientClose(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestBidiStream")

	// sending a ClientClose to the ProxyStream will also ClientClose the streams.
	err := proxyStream.CloseSend()
	tu.FatalOnErr("CloseSend()", err, t)

	// This should trigger the close of the target stream, with no error
	reply := testutil.Exchange(t, proxyStream, nil)
	sc := reply.GetServerClose()
	if sc == nil || sc.GetStatus() != nil {
		t.Fatalf("Exchange(), want ServerClose with nil status, got %+v", reply)
	}
}

func TestProxyServerNonceReusePrevention(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	req := &pb.ProxyRequest{
		Request: &pb.ProxyRequest_StartStream{
			StartStream: &pb.StartStream{
				Target:     "foo:456",
				Nonce:      1,
				MethodName: "/Testdata.TestService/TestUnary",
			},
		},
	}
	// Initial successful stream establishment
	resp := testutil.Exchange(t, proxyStream, req)
	if resp.GetStartStreamReply() == nil {
		t.Fatalf("Exchange(%v) = %T, want StartStreamReply", req, resp.Reply)
	}

	// Sending a second request with the same nonce and target is
	// a client error which will cause the proxy stream to
	// be cancelled.
	// This will yield Cancelled ServerClose messages for
	// the active stream, and a final status of "FailedPrecondition"
	// on the proxy stream.
	resp = testutil.Exchange(t, proxyStream, req)
	sc := resp.GetServerClose()
	if sc == nil || !strings.Contains(sc.GetStatus().GetMessage(), "canceled") {
		t.Fatalf("Exchange(%v) got reply %v, want ServerClose with cancel", req, resp)
	}

	// The next receive will yeild a cancellation of the initial stream
	got, err := proxyStream.Recv()
	if c := status.Code(err); c != codes.FailedPrecondition {
		t.Fatalf("Recv() = %v, err was %v, want err with FailedPrecondition", got, err)
	}
}

func TestProxyServerStartStreamAuth(t *testing.T) {
	ctx := context.Background()
	policy := `

package sansshell.authz

default allow = false

# Only allow starting streams to foo:123"
allow {
  input.method = "/Proxy.Proxy/Proxy"
  input.message.start_stream.target = "foo:123"
}
`
	authz := testutil.NewOpaRPCAuthorizer(ctx, t, policy)
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:456")
	proxyStream := startTestProxyWithAuthz(ctx, t, testServerMap, authz)

	// startStream to foo:123 is successful
	testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")

	req := &pb.ProxyRequest{
		Request: &pb.ProxyRequest_StartStream{
			StartStream: &pb.StartStream{
				Target:     "bar:456",
				MethodName: "/Testdata.TestService/TestUnary",
				Nonce:      42,
			},
		},
	}
	// Attempting to start a second stream will first yield a Cancel on the
	// initial stream as the proxy stream is torn down, followed by a
	// final error with a PermissionDenied
	reply := testutil.Exchange(t, proxyStream, req)
	t.Log(reply)
	if reply.GetServerClose() == nil {
		t.Fatalf("Exchange(%v) reply was %v, want ServerClose", req, reply)
	}

	// subsequent recv gets the final status, which is PermissionDenied
	resp, err := proxyStream.Recv()
	t.Log(resp)
	if err == nil || status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Recv() err was %v, want err with code PermissionDenied", err)
	}
}

func TestProxyServerAuthzPolicyUnary(t *testing.T) {
	ctx := context.Background()
	policy := `
package sansshell.authz

default allow = false

# Allow connections to the proxy
allow {
  input.method = "/Proxy.Proxy/Proxy"
}

# allow TestUnary with any value to foo:123
allow {
  input.method = "/Testdata.TestService/TestUnary"
  input.host.net.address = "foo"
}

# allow TestRequest with value "allowed_bar" to bar:456
allow {
  input.type = "Testdata.TestRequest"
  input.message.input = "allowed_bar"
  input.host.net.address = "bar"
}
`
	authz := testutil.NewOpaRPCAuthorizer(ctx, t, policy)
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:456")

	packReply := func(t *testing.T, reply *tdpb.TestResponse) *pb.ProxyReply {
		packed, err := anypb.New(reply)
		tu.FatalOnErr(fmt.Sprintf("anypb.New(%v)", reply), err, t)
		return &pb.ProxyReply{
			Reply: &pb.ProxyReply_StreamData{
				StreamData: &pb.StreamData{
					Payload: packed,
				},
			},
		}
	}
	for _, tc := range []struct {
		name   string
		target string
		req    *tdpb.TestRequest
		reply  *pb.ProxyReply
	}{
		{
			name:   "allowed bar",
			target: "bar:456",
			req:    &tdpb.TestRequest{Input: "allowed_bar"},
			reply:  packReply(t, &tdpb.TestResponse{Output: "bar:456 allowed_bar"}),
		},
		{
			name:   "allowed foo 1",
			target: "foo:123",
			req:    &tdpb.TestRequest{Input: "allowed_foo"},
			reply:  packReply(t, &tdpb.TestResponse{Output: "foo:123 allowed_foo"}),
		},
		{
			name:   "allowed foo 1",
			target: "foo:123",
			req:    &tdpb.TestRequest{Input: "allowed_bar"},
			reply:  packReply(t, &tdpb.TestResponse{Output: "foo:123 allowed_bar"}),
		},
		{
			name:   "denied bar",
			target: "bar:456",
			req:    &tdpb.TestRequest{Input: "denied_bar"},
			reply: &pb.ProxyReply{
				Reply: &pb.ProxyReply_ServerClose{
					ServerClose: &pb.ServerClose{
						Status: &pb.Status{
							Code:    int32(codes.PermissionDenied),
							Message: "OPA policy does not permit this request",
						},
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			proxyStream := startTestProxyWithAuthz(ctx, t, testServerMap, authz)
			streamid := testutil.MustStartStream(t, proxyStream, tc.target, "/Testdata.TestService/TestUnary")
			packed := testutil.PackStreamData(t, tc.req, streamid)
			got := testutil.Exchange(t, proxyStream, packed)
			diffOpts := []cmp.Option{
				protocmp.Transform(),
				protocmp.IgnoreFields(&pb.ServerClose{}, "stream_ids"),
				protocmp.IgnoreFields(&pb.StreamData{}, "stream_ids"),
			}
			if diff := cmp.Diff(tc.reply, got, diffOpts...); diff != "" {
				t.Errorf("%s mismatch: (-want, +got)\n%s", tc.name, diff)
			}
		})
	}
}

func TestProxyServerAuthzPolicyBidi(t *testing.T) {
	ctx := context.Background()
	policy := `
package sansshell.authz

default allow = false

allow {
  input.method = "/Proxy.Proxy/Proxy"
}

allow {
  input.method = "/Testdata.TestService/TestBidiStream"
  input.message.input = "allowed_input"
}
`
	authz := testutil.NewOpaRPCAuthorizer(ctx, t, policy)
	testServerMap := testutil.StartTestDataServers(t, "foo:456")
	proxyStream := startTestProxyWithAuthz(ctx, t, testServerMap, authz)

	streamid := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestBidiStream")

	// Initial valid request is allowed
	allowedReq := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "allowed_input"}, streamid)

	reply := testutil.Exchange(t, proxyStream, allowedReq)
	ids, _ := testutil.UnpackStreamData(t, reply)
	if len(ids) == 0 || ids[0] != streamid {
		t.Fatalf("Exchange(), want StreamData reply to stream %d, got %+v", streamid, reply)
	}

	// subsequent 'bad' request is denied.
	deniedReq := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "not allowed"}, streamid)

	reply = testutil.Exchange(t, proxyStream, deniedReq)
	sc := reply.GetServerClose()
	if sc == nil || sc.GetStatus().GetCode() != int32(codes.PermissionDenied) {
		t.Fatalf("Exchange(%v), want ServerClose with PermissionDenied, got %v", deniedReq, reply)
	}
}

func TestProxyServerAuthzPolicyPartialFailure(t *testing.T) {
	ctx := context.Background()
	// policy which permits foo:123, but not bar:456
	policy := `
package sansshell.authz

default allow = false

allow {
  input.method = "/Proxy.Proxy/Proxy"
}

allow {
  input.method = "/Testdata.TestService/TestUnary"
  input.host.net.address = "foo"
}
`
	authz := testutil.NewOpaRPCAuthorizer(ctx, t, policy)
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:456")
	proxyStream := startTestProxyWithAuthz(ctx, t, testServerMap, authz)

	fooid := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")
	barid := testutil.MustStartStream(t, proxyStream, "bar:456", "/Testdata.TestService/TestUnary")

	req := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "whatever"}, fooid, barid)

	// we'll eventually get 3 replies, but the order is undefined, so we'll collect all
	replies := []*pb.ProxyReply{
		// first reply after exchanging req
		testutil.Exchange(t, proxyStream, req),
	}
	// second and 3rd replies
	replies = append(replies, testutil.Exchange(t, proxyStream, nil))
	replies = append(replies, testutil.Exchange(t, proxyStream, nil))
	sawData := false
	for _, reply := range replies {
		switch r := reply.GetReply().(type) {
		case *pb.ProxyReply_ServerClose:
			if got, want := r.ServerClose.StreamIds[0], fooid; !sawData && got == want {
				t.Error("got serverClose on foo before receiving data")
				break
			}
			// If the above didn't error then getting a close for foo is fine.
			if r.ServerClose.StreamIds[0] == fooid {
				break
			}
			if got, want := r.ServerClose.StreamIds[0], barid; got != want {
				t.Errorf("got serverClose for wrong stream ID. Got %d want %d", got, want)
			}
			if got, want := r.ServerClose.GetStatus().GetCode(), int32(codes.PermissionDenied); got != want {
				t.Errorf("Status for bar was %v, want status with PermissionDenied", r.ServerClose.GetStatus())
			}
		case *pb.ProxyReply_StreamData:
			if got, want := r.StreamData.StreamIds[0], fooid; got != want {
				t.Errorf("got streamData for wrong stream ID. Got %d want %d", got, want)
			}
			sawData = true
		default:
			t.Errorf("got unexpected packet type: %+v", r)
		}
	}
}
