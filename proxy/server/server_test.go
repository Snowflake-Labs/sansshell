package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	pb "github.com/Snowflake-Labs/sansshell/proxy"
	td "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
)

// echoTestDataServer is a TestDataServiceServer for testing.
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

func withBufDialer(m map[string]*bufconn.Listener) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
		l := m[target]
		if l == nil {
			return nil, fmt.Errorf("no conn for target %s", target)
		}
		return l.Dial()
	})
}

const bufSize = 1024 * 1024

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

func startTestProxy(ctx context.Context, t *testing.T, targets map[string]*bufconn.Listener) pb.Proxy_ProxyClient {
	t.Helper()
	targetDialer := NewDialer(withBufDialer(targets), grpc.WithInsecure())
	lis := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	proxyServer := New(targetDialer)
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
	// cancelled.
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

	// send a half-close, and wait for final stream status.
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
