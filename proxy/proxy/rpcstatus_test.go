package proxy_test

import (
	"context"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/Snowflake-Labs/sansshell/proxy/server"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
)

// rpcStatusRecorder captures the final status gRPC reports for each server RPC. Tracing
// exporters turn that value into a span status, so it is what an operator ends up looking at.
type rpcStatusRecorder struct {
	statuses chan error
}

func (r *rpcStatusRecorder) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	return ctx
}

func (r *rpcStatusRecorder) HandleRPC(_ context.Context, rs stats.RPCStats) {
	if end, ok := rs.(*stats.End); ok {
		select {
		case r.statuses <- end.Error:
		default:
		}
	}
}

func (r *rpcStatusRecorder) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}

func (r *rpcStatusRecorder) HandleConn(context.Context, stats.ConnStats) {}

// startTestProxyRecordingStatus is startTestProxy with a stats handler attached, so a test can
// assert on the status the proxy RPC finishes with.
func startTestProxyRecordingStatus(ctx context.Context, t *testing.T, targets map[string]*bufconn.Listener) (map[string]*bufconn.Listener, *rpcStatusRecorder) {
	t.Helper()
	rec := &rpcStatusRecorder{statuses: make(chan error, 16)}
	targetDialer := server.NewDialer(testutil.WithBufDialer(targets), grpc.WithTransportCredentials(insecure.NewCredentials()))
	lis := bufconn.Listen(testutil.BufSize)
	authz := testutil.NewAllowAllRPCAuthorizer(ctx, t)
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(authz.AuthorizeStream),
		grpc.StatsHandler(rec),
	)
	proxyServer := server.New(targetDialer, authz)
	proxyServer.Register(grpcServer)
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() { grpcServer.Stop() })
	return map[string]*bufconn.Listener{"proxy": lis}, rec
}

func waitForRPCStatus(t *testing.T, rec *rpcStatusRecorder) error {
	t.Helper()
	select {
	case err := <-rec.statuses:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("the proxy RPC never finished")
		return nil
	}
}

// A completed one-to-many call should leave the proxy RPC in a successful state. Before the
// dispatch change the RPC only ended when the client disconnected, so it always finished as
// "Unavailable: transport is closing".
func TestProxyRPCFinishesCleanlyAfterInvokeOneMany(t *testing.T) {
	for _, tc := range []struct {
		name    string
		targets []string
	}{
		{name: "one target", targets: []string{"foo:123"}},
		{name: "several targets", targets: []string{"foo:123", "bar:123", "baz:123"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			targets := testutil.StartTestDataServers(t, tc.targets...)
			bufMap, rec := startTestProxyRecordingStatus(ctx, t, targets)

			conn, err := proxy.DialContext(ctx, "proxy", tc.targets,
				testutil.WithBufDialer(bufMap),
				grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatalf("DialContext: %v", err)
			}

			retChan, err := conn.InvokeOneMany(ctx, "/Testdata.TestService/TestUnary",
				&tdpb.TestRequest{Input: "hello"})
			if err != nil {
				t.Fatalf("InvokeOneMany: %v", err)
			}
			replies := 0
			for r := range retChan {
				if r.Error != nil {
					t.Fatalf("target %s returned %v", r.Target, r.Error)
				}
				replies++
			}
			if replies != len(tc.targets) {
				t.Errorf("got %d replies, want %d", replies, len(tc.targets))
			}
			if err := conn.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			if err := waitForRPCStatus(t, rec); err != nil {
				t.Errorf("proxy RPC finished with %v, want a successful status", err)
			}
		})
	}
}

// The same should hold for a streaming method, where the target stream stays open for several
// messages before it closes.
func TestProxyRPCFinishesCleanlyAfterServerStream(t *testing.T) {
	ctx := context.Background()
	targets := testutil.StartTestDataServers(t, "foo:123")
	bufMap, rec := startTestProxyRecordingStatus(ctx, t, targets)

	conn, err := proxy.DialContext(ctx, "proxy", []string{"foo:123"},
		testutil.WithBufDialer(bufMap),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}

	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true},
		"/Testdata.TestService/TestServerStream")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	if err := stream.SendMsg(&tdpb.TestRequest{Input: "hello"}); err != nil {
		t.Fatalf("SendMsg: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}
	for {
		err := stream.RecvMsg(&tdpb.TestResponse{})
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("RecvMsg: %v", err)
		}
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := waitForRPCStatus(t, rec); err != nil {
		t.Errorf("proxy RPC finished with %v, want a successful status", err)
	}
}
