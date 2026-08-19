package proxy_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
)

// dispatch now waits for target streams to report, so a client that half-closes and walks
// away must still not leave the RPC running.
func TestProxyRPCFinishesWhenClientAbandonsStream(t *testing.T) {
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
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The status may well be an error since the client is gone. Only termination matters.
	waitForRPCStatus(t, rec)
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
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := waitForRPCStatus(t, rec); err != nil {
		t.Errorf("proxy RPC finished with %v, want a successful status", err)
	}
}
