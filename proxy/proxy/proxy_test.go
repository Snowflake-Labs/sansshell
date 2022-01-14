// Needs to be in a test package as the protobuf imports proxy which would then
// cause a circular import.
package proxy_test

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/Snowflake-Labs/sansshell/proxy/server"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
	tu "github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func startTestProxy(ctx context.Context, t *testing.T, targets map[string]*bufconn.Listener) map[string]*bufconn.Listener {
	t.Helper()
	targetDialer := server.NewDialer(testutil.WithBufDialer(targets), grpc.WithTransportCredentials(insecure.NewCredentials()))
	lis := bufconn.Listen(testutil.BufSize)
	authz := testutil.NewAllowAllRpcAuthorizer(ctx, t)
	grpcServer := grpc.NewServer(grpc.StreamInterceptor(authz.AuthorizeStream))
	proxyServer := server.New(targetDialer, authz)
	proxyServer.Register(grpcServer)
	go func() {
		// Don't care about errors here as they might come on shutdown and we
		// can't log through t at that point anyways.
		grpcServer.Serve(lis)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
	})
	return map[string]*bufconn.Listener{"proxy": lis}
}

func TestDial(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:123")
	startTestProxy(ctx, t, testServerMap)

	for _, tc := range []struct {
		name    string
		proxy   string
		targets []string
		wantErr bool
	}{
		{
			name:    "proxy and N hosts",
			proxy:   "proxy",
			targets: []string{"foo:123", "bar:123"},
		},
		{
			name:    "no proxy and a host",
			targets: []string{"foo:123"},
		},
		{
			name:    "proxy and 1 host",
			proxy:   "proxy",
			targets: []string{"foo:123"},
		},
		{
			name:    "no proxy and N hosts",
			targets: []string{"foo:123", "bar:123"},
			wantErr: true,
		},
		{
			name:    "proxy and no targets",
			proxy:   "proxy",
			wantErr: true,
		},
		{
			name:    "no proxy no targets",
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := proxy.Dial(tc.proxy, tc.targets, grpc.WithTransportCredentials(insecure.NewCredentials()))
			tu.WantErr(tc.name, err, tc.wantErr, t)
		})
	}
}

func TestUnary(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:123")
	bufMap := startTestProxy(ctx, t, testServerMap)

	// Combines the 2 maps so we can dial everything directly if needed.
	for k, v := range testServerMap {
		bufMap[k] = v
	}

	for _, tc := range []struct {
		name           string
		proxy          string
		targets        []string
		wantErrOneMany bool
		wantErr        bool
	}{
		{
			name:    "proxy N targets",
			proxy:   "proxy",
			targets: []string{"foo:123", "bar:123"},
			wantErr: true, // can't do unary call with N targets
		},
		{
			name:    "proxy 1 target",
			proxy:   "proxy",
			targets: []string{"foo:123"},
		},
		{
			name:    "no proxy 1 target",
			targets: []string{"foo:123"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conn, err := proxy.Dial(tc.proxy, tc.targets, testutil.WithBufDialer(bufMap), grpc.WithTransportCredentials(insecure.NewCredentials()))
			tu.FatalOnErr("Dial", err, t)

			ts := tdpb.NewTestServiceClientProxy(conn)
			resp, err := ts.TestUnaryOneMany(context.Background(), &tdpb.TestRequest{Input: "input"})
			t.Log(err)
			tu.WantErr(tc.name, err, tc.wantErrOneMany, t)

			if !tc.wantErrOneMany {
				for r := range resp {
					tu.FatalOnErr(fmt.Sprintf("target %s", r.Target), r.Error, t)
				}
			}

			_, err = ts.TestUnary(context.Background(), &tdpb.TestRequest{Input: "input"})
			t.Log(err)
			tu.WantErr(tc.name, err, tc.wantErr, t)

			_, err = ts.TestUnaryOneMany(context.Background(), &tdpb.TestRequest{Input: "error"})
			tu.FatalOnErr("TestUnaryOneMany error", err, t)
			for r := range resp {
				tu.FatalOnNoErr(fmt.Sprintf("target %s", r.Target), r.Error, t)
			}

			// Check pass through cases
			if tc.proxy != "" {
				_, err = ts.TestUnary(context.Background(), &tdpb.TestRequest{Input: "error"})
				tu.FatalOnNoErr("TestUnary error", err, t)
			}

			// Do some direct calls against the conn to get at error cases
			_, err = conn.InvokeOneMany(context.Background(), "bad_method", &tdpb.TestRequest{Input: "input"})
			tu.FatalOnNoErr("InvokeOneMany bad method", err, t)
			_, err = conn.InvokeOneMany(context.Background(), "/Testdata.TestService/TestUnary", nil)
			tu.FatalOnNoErr("InvokeOneMany bad msg", err, t)

			err = conn.Close()
			tu.FatalOnErr("conn Close()", err, t)
		})
	}
}

func TestStreaming(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:123")
	bufMap := startTestProxy(ctx, t, testServerMap)

	// Combines the 2 maps so we can dial everything directly if needed.
	for k, v := range testServerMap {
		bufMap[k] = v
	}

	for _, tc := range []struct {
		name           string
		proxy          string
		targets        []string
		wantErrOneMany bool
		wantErr        bool
	}{
		{
			name:    "proxy N targets",
			proxy:   "proxy",
			targets: []string{"foo:123", "bar:123"},
			wantErr: true,
		},
		{
			name:    "proxy 1 target",
			proxy:   "proxy",
			targets: []string{"foo:123"},
		},
		{
			name:    "no proxy 1 target",
			targets: []string{"foo:123"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conn, err := proxy.Dial(tc.proxy, tc.targets, testutil.WithBufDialer(bufMap), grpc.WithTransportCredentials(insecure.NewCredentials()))
			tu.FatalOnErr("Dial", err, t)

			ts := tdpb.NewTestServiceClientProxy(conn)
			stream, err := ts.TestBidiStreamOneMany(context.Background())
			tu.FatalOnErr("getting stream", err, t)

			// Should always fail for proxy. For direct these may hang up so don't bother.
			if tc.proxy != "" {
				_, err = stream.Header()
				tu.FatalOnNoErr("Header on proxy stream", err, t)

				// Should always return nil for proxy
				if td := stream.Trailer(); td != nil {
					t.Fatalf("unexpected response from Trailer(): %v", td)
				}
			}

			// In the proxy case do some bad messages which won't send anything
			if tc.proxy != "" {
				err = stream.SendMsg(nil)
				tu.FatalOnNoErr("SendMsg with nil", err, t)

				err = stream.RecvMsg(nil)
				tu.FatalOnNoErr("RecvMsg with nil", err, t)
			}

			// Should return a context
			if c := stream.Context(); c == nil {
				t.Fatal("Context() returned nil")
			}

			// Send one normal and one error. We should get back 4 replies, # targets errors in that.
			err = stream.Send(&tdpb.TestRequest{Input: "input"})
			tu.FatalOnErr("Send", err, t)
			err = stream.Send(&tdpb.TestRequest{Input: "error"})
			tu.FatalOnErr("Send error", err, t)

			errors := 0

			for {
				resp, err := stream.Recv()
				if err != nil && err == io.EOF {
					break
				}
				tu.FatalOnErr("Recv", err, t)
				for _, r := range resp {
					t.Logf("%+v", r)
					if r.Error == io.EOF {
						continue
					}
					if r.Error != nil {
						errors++
					}
				}
			}

			if got, want := errors, len(tc.targets); got != want {
				t.Fatalf("Invalid error count, got %d want %d", got, want)
			}

			// Shouldn't fail.
			err = stream.CloseSend()
			tu.FatalOnErr("CloseSend", err, t)

			// Should give an error now
			err = stream.Send(&tdpb.TestRequest{Input: "input"})
			tu.FatalOnNoErr("Send on closed", err, t)
			conn.Close()
		})
	}

}
