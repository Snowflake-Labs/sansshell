package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	pb "github.com/Snowflake-Labs/sansshell/proxy"
	td "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
)

// echoTestDataServer is a TestDataServiceServer for testing
type echoTestDataServer struct {
	serverName string
}

func (e *echoTestDataServer) TestUnary(ctx context.Context, req *td.TestRequest) (*td.TestResponse, error) {
	return &td.TestResponse{
		Output: fmt.Sprintf("%s %s", e.serverName, req.Input),
	}, nil
}

func (e *echoTestDataServer) TestServerStream(req *td.TestRequest, stream td.TestService_TestServerStreamServer) error {
	for i := 0; i < 5; i++ {
		stream.Send(&td.TestResponse{
			Output: fmt.Sprintf("%s %d %s", e.serverName, i, req.Input),
		})
	}
	return nil
}

func (e *echoTestDataServer) TestClientStream(stream td.TestService_TestClientStreamServer) error {
	var inputs []string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&td.TestResponse{
				Output: fmt.Sprintf("%s %s", e.serverName, strings.Join(inputs, ",")),
			})
		}
		if err != nil {
			return err
		}
		inputs = append(inputs, req.Input)
	}
}

func (e *echoTestDataServer) TestBidiStream(stream td.TestService_TestBidiStreamServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(&td.TestResponse{
			Output: fmt.Sprintf("%s %s", e.serverName, req.Input),
		}); err != nil {
			return nil
		}
	}
}

func newRpcAuthorizer(ctx context.Context, t *testing.T, policy string) *rpcauth.Authorizer {
	t.Helper()
	auth, err := rpcauth.NewWithPolicy(ctx, policy)
	if err != nil {
		t.Fatalf("rpcauth.NewWithPolicy(%s) err was %v, want nil", policy, err)
	}
	return auth
}

func newAllowAllRpcAuthorizer(ctx context.Context, t *testing.T) *rpcauth.Authorizer {
	policy := `
package sansshell.authz
default allow = true
`
	return newRpcAuthorizer(ctx, t, policy)
}

func withBufDialer(m map[string]*bufconn.Listener) grpc.DialOption {
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

const bufSize = 1024 * 1024

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

func startTestDataServer(t *testing.T, serverName string) *bufconn.Listener {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	echoServer := &echoTestDataServer{serverName: serverName}
	rpcServer := grpc.NewServer()
	td.RegisterTestServiceServer(rpcServer, echoServer)
	go func() {
		err := rpcServer.Serve(lis)
		if err != nil {
			t.Errorf("%s %s", serverName, err)
		}
	}()
	t.Cleanup(func() {
		rpcServer.Stop()
	})
	return lis
}

func startTestDataServers(t *testing.T, serverNames ...string) map[string]*bufconn.Listener {
	t.Helper()
	out := map[string]*bufconn.Listener{}
	for _, name := range serverNames {
		out[name] = startTestDataServer(t, name)
	}
	return out
}

func startTestProxyWithAuthz(ctx context.Context, t *testing.T, targets map[string]*bufconn.Listener, authz *rpcauth.Authorizer) pb.Proxy_ProxyClient {
	t.Helper()
	targetDialer := NewDialer(withBufDialer(targets), grpc.WithInsecure())
	lis := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer(grpc.StreamInterceptor(authz.AuthorizeStream))
	proxyServer := New(targetDialer, authz)
	proxyServer.Register(grpcServer)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Errorf("proxy: %v", err)
		}
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
	})
	bufMap := map[string]*bufconn.Listener{"proxy": lis}
	conn, err := grpc.DialContext(ctx, "proxy", withBufDialer(bufMap), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("DialContext(proxy), err was %v, want nil", err)
	}
	stream, err := pb.NewProxyClient(conn).Proxy(ctx)
	if err != nil {
		t.Fatalf("proxy.Proxy(), err was %v, want nil", err)
	}
	return stream
}

func startTestProxy(ctx context.Context, t *testing.T, targets map[string]*bufconn.Listener) pb.Proxy_ProxyClient {
	return startTestProxyWithAuthz(ctx, t, targets, newAllowAllRpcAuthorizer(ctx, t))
}

func TestProxyServerStartStream(t *testing.T) {
	ctx := context.Background()
	testServerMap := startTestDataServers(t, "foo:123", "bar:123")
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
			if c := reply.GetErrorStatus().GetCode(); codes.Code(c) != codes.Internal {
				t.Errorf("startTargetStream(%s, %s) reply was %v, want reply with code Internal", name, m, reply)
			}
		}
	}
}

func TestProxyServerCancel(t *testing.T) {
	// Test that streams can be cancelled
	ctx := context.Background()
	testServerMap := startTestDataServers(t, "foo:123", "bar:456")
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
	if err := proxyStream.Send(cancelReq); err != nil {
		t.Fatalf("ClientCancel(foo, bar) err was %v, want nil", err)
	}

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
	testServerMap := startTestDataServers(t, "foo:123")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamId := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")

	req := testutil.PackStreamData(t, &td.TestRequest{Input: "Foo"}, streamId)

	reply := testutil.Exchange(t, proxyStream, req)
	ids, data := testutil.UnpackStreamData(t, reply)
	if ids[0] != streamId {
		t.Errorf("StreamData.StreamIds[0] = %d, want %d", ids[0], streamId)
	}
	want := &td.TestResponse{
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
	testServerMap := startTestDataServers(t, "foo:123", "bar:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	fooId := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")
	barId := testutil.MustStartStream(t, proxyStream, "bar:456", "/Testdata.TestService/TestUnary")

	req := testutil.PackStreamData(t, &td.TestRequest{Input: "Foo"}, fooId, barId)

	if err := proxyStream.Send(req); err != nil {
		t.Fatalf("Send(%v) err was %v, want nil", req, err)
	}

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
			case fooId:
				// expected
			case barId:
				// expected
			default:
				t.Fatalf("got response with id %d, want one of (%d, %d)", ids[0], fooId, barId)
			}
			if sc.GetStatus() != nil {
				t.Fatalf("got ServerClose with error %v, want nil", sc.GetStatus())
			}
		case *pb.ProxyReply_StreamData:
			ids, data := testutil.UnpackStreamData(t, reply)
			want := &td.TestResponse{}
			switch ids[0] {
			case fooId:
				want.Output = "foo:123 Foo"
			case barId:
				want.Output = "bar:456 Foo"
			default:
				t.Fatalf("got response with id %d, want one of (%d, %d)", ids[0], fooId, barId)
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
	testServerMap := startTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamId := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestServerStream")

	req := testutil.PackStreamData(t, &td.TestRequest{Input: "Foo"}, streamId)
	reply := testutil.Exchange(t, proxyStream, req)
	replies := []*pb.ProxyReply{reply}

	for i := 0; i < 4; i++ {
		replies = append(replies, testutil.Exchange(t, proxyStream, nil))
	}
	for i := 0; i < 5; i++ {
		want := &td.TestResponse{
			Output: fmt.Sprintf("foo:456 %d Foo", i),
		}
		_, data := testutil.UnpackStreamData(t, replies[i])
		if !proto.Equal(want, data) {
			t.Errorf("Server response %d, proto.Equal(%v, %v) was false, want true", i, want, data)
		}
	}
}

func TestProxyServerClientStream(t *testing.T) {
	ctx := context.Background()

	testServerMap := startTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamId := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestClientStream")

	req := testutil.PackStreamData(t, &td.TestRequest{Input: "Foo"}, streamId)

	for i := 0; i < 3; i++ {
		if err := proxyStream.Send(req); err != nil {
			t.Fatalf("%d : proxyStream.Send(%v), err was %v, want nil", i, req, err)
		}
	}

	// Send a half-close, and exchange it for the reply
	hc := &pb.ProxyRequest{
		Request: &pb.ProxyRequest_ClientClose{
			ClientClose: &pb.ClientClose{
				StreamIds: []uint64{streamId},
			},
		},
	}

	want := &td.TestResponse{
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
	testServerMap := startTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamId := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestBidiStream")

	req := testutil.PackStreamData(t, &td.TestRequest{Input: "Foo"}, streamId)

	want := &td.TestResponse{
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
				StreamIds: []uint64{streamId},
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
	testServerMap := startTestDataServers(t, "foo:456")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestBidiStream")

	// sending a ClientClose to the ProxyStream will also ClientClose the streams.
	if err := proxyStream.CloseSend(); err != nil {
		t.Fatalf("CloseSend(); err was %v, want nil", err)
	}

	// This should trigger the close of the target stream, with no error
	reply := testutil.Exchange(t, proxyStream, nil)
	sc := reply.GetServerClose()
	if sc == nil || sc.GetStatus() != nil {
		t.Fatalf("Exchange(), want ServerClose with nil status, got %+v", reply)
	}
}

func TestProxyServerNonceReusePrevention(t *testing.T) {
	ctx := context.Background()
	testServerMap := startTestDataServers(t, "foo:456")
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
	authz := newRpcAuthorizer(ctx, t, policy)
	testServerMap := startTestDataServers(t, "foo:123", "bar:456")
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
	if reply.GetServerClose() == nil {
		t.Fatalf("Exchange(%v) reply was %v, want ServerClose", req, reply)
	}

	// subsequent recv gets the final status, which is PermissionDenied
	_, err := proxyStream.Recv()
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
	authz := newRpcAuthorizer(ctx, t, policy)
	testServerMap := startTestDataServers(t, "foo:123", "bar:456")

	packReply := func(t *testing.T, reply *td.TestResponse) *pb.ProxyReply {
		packed, err := anypb.New(reply)
		if err != nil {
			t.Fatalf("anypby.New(%v) err was %v, want nil", reply, err)
		}
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
		req    *td.TestRequest
		reply  *pb.ProxyReply
	}{
		{
			name:   "allowed bar",
			target: "bar:456",
			req:    &td.TestRequest{Input: "allowed_bar"},
			reply:  packReply(t, &td.TestResponse{Output: "bar:456 allowed_bar"}),
		},
		{
			name:   "allowed foo 1",
			target: "foo:123",
			req:    &td.TestRequest{Input: "allowed_foo"},
			reply:  packReply(t, &td.TestResponse{Output: "foo:123 allowed_foo"}),
		},
		{
			name:   "allowed foo 1",
			target: "foo:123",
			req:    &td.TestRequest{Input: "allowed_bar"},
			reply:  packReply(t, &td.TestResponse{Output: "foo:123 allowed_bar"}),
		},
		{
			name:   "denied bar",
			target: "bar:456",
			req:    &td.TestRequest{Input: "denied_bar"},
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
	authz := newRpcAuthorizer(ctx, t, policy)
	testServerMap := startTestDataServers(t, "foo:456")
	proxyStream := startTestProxyWithAuthz(ctx, t, testServerMap, authz)

	streamid := testutil.MustStartStream(t, proxyStream, "foo:456", "/Testdata.TestService/TestBidiStream")

	// Initial valid request is allowed
	allowedReq := testutil.PackStreamData(t, &td.TestRequest{Input: "allowed_input"}, streamid)

	reply := testutil.Exchange(t, proxyStream, allowedReq)
	ids, _ := testutil.UnpackStreamData(t, reply)
	if len(ids) == 0 || ids[0] != streamid {
		t.Fatalf("Exchange(), want StreamData reply to stream %d, got %+v", streamid, reply)
	}

	// subsequent 'bad' request is denied.
	deniedReq := testutil.PackStreamData(t, &td.TestRequest{Input: "not allowed"}, streamid)

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
	authz := newRpcAuthorizer(ctx, t, policy)
	testServerMap := startTestDataServers(t, "foo:123", "bar:456")
	proxyStream := startTestProxyWithAuthz(ctx, t, testServerMap, authz)

	fooid := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")
	barid := testutil.MustStartStream(t, proxyStream, "bar:456", "/Testdata.TestService/TestUnary")

	req := testutil.PackStreamData(t, &td.TestRequest{Input: "whatever"}, fooid, barid)

	// we'll eventually get 2 replies, but the order is undefined, so we'll collect both
	replies := []*pb.ProxyReply{
		// first reply after exchanging req
		testutil.Exchange(t, proxyStream, req),
	}
	// second reply
	replies = append(replies, testutil.Exchange(t, proxyStream, nil))
	for _, reply := range replies {
		sc := reply.GetServerClose()
		if sc == nil || len(sc.StreamIds) != 1 {
			t.Fatalf("Proxy reply was %v, want ServerClose with single streamID", reply)
		}
		switch sc.StreamIds[0] {
		case barid:
			// barID is not permitted by the policy, we we get a PermissionDenied
			if sc.GetStatus().GetCode() != int32(codes.PermissionDenied) {
				t.Errorf("Status for bar was %v, want status with PermissionDenied", sc.GetStatus())
			}
		case fooid:
			// fooID is permitted, but won't be sent due to the failure in bar
			if sc.GetStatus().GetCode() != int32(codes.Aborted) {
				t.Errorf("Status for foo was %v, want status with Aborted", sc.GetStatus())
			}
		default:
			t.Errorf("Got ServerClose with streamid %d, want streamid in [%d, %d]", sc.StreamIds[0], fooid, barid)
		}
	}
}
