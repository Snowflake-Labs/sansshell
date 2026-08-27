/* Copyright (c) 2026 Snowflake Inc. All rights reserved.

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

package proxy_test

import (
	"context"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
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
		// Dropping is deliberate. A blocking send inside a gRPC callback would stall the
		// server, and the tests only ever read the first status.
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

func dialTestProxy(ctx context.Context, t *testing.T, targets []string) (*proxy.Conn, *rpcStatusRecorder) {
	t.Helper()
	rec := &rpcStatusRecorder{statuses: make(chan error, 16)}
	bufMap := startTestProxy(ctx, t, testutil.StartTestDataServers(t, targets...), grpc.StatsHandler(rec))
	conn, err := proxy.DialContext(ctx, "proxy", targets,
		testutil.WithBufDialer(bufMap),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	return conn, rec
}

func waitForRPCStatus(t *testing.T, rec *rpcStatusRecorder) error {
	t.Helper()
	select {
	case err := <-rec.statuses:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the proxy RPC never finished")
		return nil
	}
}

func drainToEOF(t *testing.T, stream grpc.ClientStream) {
	t.Helper()
	for {
		err := stream.RecvMsg(&tdpb.TestResponse{})
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("RecvMsg: %v", err)
		}
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
			conn, rec := dialTestProxy(ctx, t, tc.targets)

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
			// Assert before closing: the RPC has to finish on its own, not because the
			// client went away.
			if err := waitForRPCStatus(t, rec); err != nil {
				t.Errorf("proxy RPC finished with %v, want a successful status", err)
			}
			if err := conn.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

// The same should hold for a streaming method, where the target stream stays open for several
// messages before it closes.
func TestProxyRPCFinishesCleanlyAfterServerStream(t *testing.T) {
	ctx := context.Background()
	conn, rec := dialTestProxy(ctx, t, []string{"foo:123"})

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
	drainToEOF(t, stream)
	// Assert before closing: the RPC has to finish on its own, not because the client
	// went away.
	if err := waitForRPCStatus(t, rec); err != nil {
		t.Errorf("proxy RPC finished with %v, want a successful status", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// The target is still sending when the client half-closes, so dispatch drains for longer here.
func TestProxyRPCFinishesCleanlyAfterBidiStream(t *testing.T) {
	ctx := context.Background()
	conn, rec := dialTestProxy(ctx, t, []string{"foo:123"})

	stream, err := conn.NewStream(ctx,
		&grpc.StreamDesc{ClientStreams: true, ServerStreams: true},
		"/Testdata.TestService/TestBidiStream")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := stream.SendMsg(&tdpb.TestRequest{Input: "hello"}); err != nil {
			t.Fatalf("SendMsg: %v", err)
		}
		if err := stream.RecvMsg(&tdpb.TestResponse{}); err != nil {
			t.Fatalf("RecvMsg: %v", err)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}
	drainToEOF(t, stream)
	// Assert before closing: the RPC has to finish on its own, not because the client
	// went away.
	if err := waitForRPCStatus(t, rec); err != nil {
		t.Errorf("proxy RPC finished with %v, want a successful status", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// Here the target only replies once the client half-closes, so it finishes right after
// ClientCloseAll rather than well before it.
func TestProxyRPCFinishesCleanlyAfterClientStream(t *testing.T) {
	ctx := context.Background()
	conn, rec := dialTestProxy(ctx, t, []string{"foo:123"})

	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ClientStreams: true},
		"/Testdata.TestService/TestClientStream")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := stream.SendMsg(&tdpb.TestRequest{Input: "hello"}); err != nil {
			t.Fatalf("SendMsg: %v", err)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}
	drainToEOF(t, stream)
	// Assert before closing: the RPC has to finish on its own, not because the client
	// went away.
	if err := waitForRPCStatus(t, rec); err != nil {
		t.Errorf("proxy RPC finished with %v, want a successful status", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// A target failure must still reach the caller, and must not taint the proxy's own status.
func TestProxyRPCFinishesCleanlyWhenTargetErrors(t *testing.T) {
	ctx := context.Background()
	conn, rec := dialTestProxy(ctx, t, []string{"foo:123"})

	retChan, err := conn.InvokeOneMany(ctx, "/Testdata.TestService/TestUnary",
		&tdpb.TestRequest{Input: "error"})
	if err != nil {
		t.Fatalf("InvokeOneMany: %v", err)
	}
	errors := 0
	for r := range retChan {
		if r.Error != nil {
			errors++
			if got := status.Code(r.Error); got != codes.Unknown {
				t.Errorf("target error code = %s, want %s", got, codes.Unknown)
			}
		}
	}
	if errors != 1 {
		t.Errorf("got %d target errors, want 1", errors)
	}
	// Assert before closing: the RPC has to finish on its own, not because the client
	// went away.
	if err := waitForRPCStatus(t, rec); err != nil {
		t.Errorf("proxy RPC finished with %v, want a successful status", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
