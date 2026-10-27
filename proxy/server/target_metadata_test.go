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

package server

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	pb "github.com/Snowflake-Labs/sansshell/proxy"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
)

const targetMetadataKey = "x-target-metadata-test"

type allowAllPolicy struct{}

func (allowAllPolicy) Eval(context.Context, *rpcauth.RPCAuthInput) (bool, error) { return true, nil }

func (allowAllPolicy) DenialHints(context.Context, *rpcauth.RPCAuthInput) ([]string, error) {
	return nil, nil
}

func authorizerSettingTargetMetadata(value func(*rpcauth.RPCAuthInput) string) rpcauth.RPCAuthorizer {
	return rpcauth.NewRPCAuthorizer(allowAllPolicy{}, rpcauth.RPCAuthzHookFunc(func(_ context.Context, input *rpcauth.RPCAuthInput) error {
		if strings.HasPrefix(input.Method, "/Testdata.") {
			input.TargetMetadata = metadata.Pairs(targetMetadataKey, value(input))
		}
		return nil
	}))
}

func startMetadataRecordingTarget(t *testing.T) (*bufconn.Listener, chan []string) {
	t.Helper()
	seen := make(chan []string, 10)
	record := func(ctx context.Context) {
		md, _ := metadata.FromIncomingContext(ctx)
		seen <- md.Get(targetMetadataKey)
	}
	lis := bufconn.Listen(testutil.BufSize)
	rpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			record(ctx)
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			record(ss.Context())
			return handler(srv, ss)
		}),
	)
	tdpb.RegisterTestServiceServer(rpcServer, &testutil.EchoTestDataServer{})
	go func() { _ = rpcServer.Serve(lis) }()
	t.Cleanup(rpcServer.Stop)
	return lis, seen
}

func clientClose(streamID uint64) *pb.ProxyRequest {
	return &pb.ProxyRequest{Request: &pb.ProxyRequest_ClientClose{ClientClose: &pb.ClientClose{StreamIds: []uint64{streamID}}}}
}

func TestProxyServerSendsTargetMetadata(t *testing.T) {
	for _, method := range []string{
		"/Testdata.TestService/TestUnary",
		"/Testdata.TestService/TestServerStream",
		"/Testdata.TestService/TestBidiStream",
	} {
		t.Run(method, func(t *testing.T) {
			ctx := context.Background()
			lis, seen := startMetadataRecordingTarget(t)
			authz := authorizerSettingTargetMetadata(func(*rpcauth.RPCAuthInput) string { return "decided" })
			proxyStream := startTestProxyWithAuthz(ctx, t, map[string]*bufconn.Listener{"foo:123": lis}, authz)

			streamID := testutil.MustStartStream(t, proxyStream, "foo:123", method)
			reply := testutil.Exchange(t, proxyStream, testutil.PackStreamData(t, &tdpb.TestRequest{Input: "Foo"}, streamID))
			if reply.GetStreamData() == nil {
				t.Fatalf("want StreamData reply, got %v", reply)
			}
			if got := <-seen; len(got) != 1 || got[0] != "decided" {
				t.Errorf("target received %s = %v, want [decided]", targetMetadataKey, got)
			}
		})
	}
}

func TestProxyServerOpensClientStreamWithoutRequests(t *testing.T) {
	ctx := context.Background()
	lis, seen := startMetadataRecordingTarget(t)
	authz := authorizerSettingTargetMetadata(func(*rpcauth.RPCAuthInput) string { return "decided" })
	proxyStream := startTestProxyWithAuthz(ctx, t, map[string]*bufconn.Listener{"foo:123": lis}, authz)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestClientStream")
	reply := testutil.Exchange(t, proxyStream, clientClose(streamID))
	if reply.GetStreamData() == nil {
		t.Fatalf("want StreamData reply, got %v", reply)
	}
	if got := <-seen; len(got) != 0 {
		t.Errorf("target received %s = %v, want none because no request was authorized", targetMetadataKey, got)
	}
}

func TestProxyServerRejectsChangedTargetMetadata(t *testing.T) {
	ctx := context.Background()
	lis, seen := startMetadataRecordingTarget(t)
	authz := authorizerSettingTargetMetadata(func(input *rpcauth.RPCAuthInput) string {
		if strings.Contains(string(input.Message), "second") {
			return "changed"
		}
		return "decided"
	})
	proxyStream := startTestProxyWithAuthz(ctx, t, map[string]*bufconn.Listener{"foo:123": lis}, authz)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestBidiStream")
	if reply := testutil.Exchange(t, proxyStream, testutil.PackStreamData(t, &tdpb.TestRequest{Input: "first"}, streamID)); reply.GetStreamData() == nil {
		t.Fatalf("want StreamData reply to the first request, got %v", reply)
	}
	<-seen

	reply := testutil.Exchange(t, proxyStream, testutil.PackStreamData(t, &tdpb.TestRequest{Input: "second"}, streamID))
	if code := reply.GetServerClose().GetStatus().GetCode(); code != int32(codes.PermissionDenied) {
		t.Fatalf("want ServerClose with PermissionDenied, got %v", reply)
	}
}

func TestProxyServerTargetMetadataNeedsCredentialsDialer(t *testing.T) {
	ctx := context.Background()
	lis, _ := startMetadataRecordingTarget(t)
	targets := map[string]*bufconn.Listener{"foo:123": lis}
	authz := authorizerSettingTargetMetadata(func(*rpcauth.RPCAuthInput) string { return "decided" })
	dialer := NewDialer(testutil.WithBufDialer(targets), grpc.WithTransportCredentials(insecure.NewCredentials()))
	proxyStream := startTestProxyWithDialer(ctx, t, dialer, authz)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")
	reply := testutil.Exchange(t, proxyStream, testutil.PackStreamData(t, &tdpb.TestRequest{Input: "Foo"}, streamID))
	if code := reply.GetServerClose().GetStatus().GetCode(); code != int32(codes.FailedPrecondition) {
		t.Fatalf("want ServerClose with FailedPrecondition, got %v", reply)
	}
}
