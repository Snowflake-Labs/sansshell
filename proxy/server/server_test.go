package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/Snowflake-Labs/sansshell/proxy"
	td "github.com/Snowflake-Labs/sansshell/proxy/server/testdata"
)

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

func TestProxyServer(t *testing.T) {
	testServerMap := map[string]*bufconn.Listener{}
	for _, name := range []string{"foo:1234", "bar:1234"} {
		testServerMap[name] = startTestDataServer(t, name)
	}

	targetDialer := NewDialer(withBufDialer(testServerMap), grpc.WithInsecure())

	proxyLis := bufconn.Listen(1024 * 1024)
	proxyGrpc := grpc.NewServer()
	proxyServer := New(targetDialer)
	proxyServer.Register(proxyGrpc)

	go func() {
		if err := proxyGrpc.Serve(proxyLis); err != nil {
			t.Errorf("proxy: %v", err)
		}
	}()

	t.Cleanup(func() {
		proxyGrpc.Stop()
	})

	bufMap := map[string]*bufconn.Listener{"proxy": proxyLis}
	proxyConn, err := grpc.DialContext(context.Background(), "proxy", withBufDialer(bufMap), grpc.WithInsecure())

	if err != nil {
		t.Fatal(err)
	}

	proxyClient := pb.NewProxyClient(proxyConn)
	proxyStream, err := proxyClient.Proxy(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	streams := map[string]uint64{}

	nonce := uint32(1)
	for name := range testServerMap {
		if err := proxyStream.Send(&pb.ProxyRequest{
			Request: &pb.ProxyRequest_StartStream{
				StartStream: &pb.StartStream{
					Target:     name,
					Nonce:      nonce,
					MethodName: "/Testdata.TestService/TestUnary",
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
		rep, err := proxyStream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		streamId := rep.GetStartStreamReply().GetStreamId()
		if streamId == 0 {
			t.Fatal(fmt.Errorf("errors string stream for %s", name))
		}
		streams[name] = streamId
		nonce++
	}
	fmt.Println("streams: ", streams)
}
