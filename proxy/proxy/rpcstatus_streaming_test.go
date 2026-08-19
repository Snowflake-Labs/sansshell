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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
)

func dialTestProxy(ctx context.Context, t *testing.T, targets []string) (*proxy.Conn, *rpcStatusRecorder) {
	t.Helper()
	targetListeners := testutil.StartTestDataServers(t, targets...)
	bufMap, rec := startTestProxyRecordingStatus(ctx, t, targetListeners)
	conn, err := proxy.DialContext(ctx, "proxy", targets,
		testutil.WithBufDialer(bufMap),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	return conn, rec
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

	const messages = 5
	for i := 0; i < messages; i++ {
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
	for {
		err := stream.RecvMsg(&tdpb.TestResponse{})
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("RecvMsg after CloseSend: %v", err)
		}
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := waitForRPCStatus(t, rec); err != nil {
		t.Errorf("proxy RPC finished with %v, want a successful status", err)
	}
}

// Here the target only replies once the client half-closes, so it finishes right after
// ClientCloseAll rather than well before it.
func TestProxyRPCFinishesCleanlyAfterClientStream(t *testing.T) {
	ctx := context.Background()
	conn, rec := dialTestProxy(ctx, t, []string{"foo:123"})

	stream, err := conn.NewStream(ctx,
		&grpc.StreamDesc{ClientStreams: true},
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
