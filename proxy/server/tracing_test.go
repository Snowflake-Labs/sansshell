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
	"testing"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	pb "github.com/Snowflake-Labs/sansshell/proxy"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
)

func setupTracing(t *testing.T) (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	restore := setTracer(tp.Tracer(tracerName))
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		restore()
	})
	return exporter, tp
}

// runTargetStream creates a TargetStream via NewTargetStream and executes a
// unary RPC by calling Run() directly.  A buffered replyChan is used so that
// the unconditional send at the end of Run() never blocks, sidestepping the
// pre-existing dispatch-level deadlock that surfaces when going through the
// full proxy pipeline.
func runTargetStream(t *testing.T, targets map[string]*bufconn.Listener, target, method string, authz rpcauth.RPCAuthorizer, req *tdpb.TestRequest) {
	t.Helper()
	ctx := context.Background()

	dialer := NewDialer(testutil.WithBufDialer(targets), grpc.WithTransportCredentials(insecure.NewCredentials()))
	svcMap := LoadGlobalServiceMap()
	svcMethod := svcMap[method]
	if svcMethod == nil {
		t.Fatalf("unknown service method %s", method)
	}

	peerInfo := &rpcauth.PeerAuthInput{
		Net: &rpcauth.NetAuthInput{Network: "bufconn", Address: "test-caller"},
	}
	ctx = rpcauth.AddPeerToContext(ctx, peerInfo)

	ts, err := NewTargetStream(ctx, target, dialer, nil, svcMethod, authz, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.Send(req); err != nil {
		t.Fatal(err)
	}
	ts.CloseSend()

	replyChan := make(chan *pb.ProxyReply, 10)
	ts.Run(1, replyChan)
}

// ---------- span assertion helpers ----------

func findSpan(spans tracetest.SpanStubs, name string) *tracetest.SpanStub {
	for i := range spans {
		if spans[i].Name == name {
			return &spans[i]
		}
	}
	return nil
}

func findTargetSpan(spans tracetest.SpanStubs, name, target string) *tracetest.SpanStub {
	for i := range spans {
		if spans[i].Name == name && spanHasAttribute(&spans[i], "sansshell.target.address", target) {
			return &spans[i]
		}
	}
	return nil
}

func spanHasAttribute(span *tracetest.SpanStub, key, value string) bool {
	for _, attr := range span.Attributes {
		if string(attr.Key) == key && attr.Value.AsString() == value {
			return true
		}
	}
	return false
}

func spanHasEvent(span *tracetest.SpanStub, name string) bool {
	for _, ev := range span.Events {
		if ev.Name == name {
			return true
		}
	}
	return false
}

func eventHasAttribute(span *tracetest.SpanStub, eventName, key, value string) bool {
	for _, ev := range span.Events {
		if ev.Name == eventName {
			for _, attr := range ev.Attributes {
				if string(attr.Key) == key && attr.Value.AsString() == value {
					return true
				}
			}
		}
	}
	return false
}

// ---------- tests ----------

func TestTracing_TargetSpanCreated(t *testing.T) {
	exporter, _ := setupTracing(t)
	testServerMap := testutil.StartTestDataServers(t, "foo:123")
	authz := testutil.NewAllowAllRPCAuthorizer(context.Background(), t)

	runTargetStream(t, testServerMap, "foo:123", "/Testdata.TestService/TestUnary", authz, &tdpb.TestRequest{Input: "hello"})

	spans := exporter.GetSpans()
	targetSpan := findTargetSpan(spans, "proxy.target/Testdata.TestService/TestUnary", "foo:123")
	if targetSpan == nil {
		t.Fatalf("expected target span 'proxy.target/Testdata.TestService/TestUnary' with target foo:123 not found in %d span(s)", len(spans))
	}
	if !spanHasAttribute(targetSpan, "sansshell.target.method", "/Testdata.TestService/TestUnary") {
		t.Error("target span missing sansshell.target.method")
	}
	if !spanHasAttribute(targetSpan, "sansshell.target.stream_type", "unary") {
		t.Error("target span missing sansshell.target.stream_type=unary")
	}
}

func TestTracing_DialSubSpan(t *testing.T) {
	exporter, _ := setupTracing(t)
	testServerMap := testutil.StartTestDataServers(t, "foo:123")
	authz := testutil.NewAllowAllRPCAuthorizer(context.Background(), t)

	runTargetStream(t, testServerMap, "foo:123", "/Testdata.TestService/TestUnary", authz, &tdpb.TestRequest{Input: "x"})

	spans := exporter.GetSpans()
	dialSpan := findTargetSpan(spans, "proxy.target.dial", "foo:123")
	if dialSpan == nil {
		t.Fatal("expected dial span 'proxy.target.dial' with target foo:123 not found")
	}
	if dialSpan.Status.Code != 0 {
		t.Errorf("dial span has unexpected error status: %v", dialSpan.Status)
	}
}

func TestTracing_StreamEvents(t *testing.T) {
	exporter, _ := setupTracing(t)
	testServerMap := testutil.StartTestDataServers(t, "foo:123")
	authz := testutil.NewAllowAllRPCAuthorizer(context.Background(), t)

	runTargetStream(t, testServerMap, "foo:123", "/Testdata.TestService/TestUnary", authz, &tdpb.TestRequest{Input: "hi"})

	spans := exporter.GetSpans()
	targetSpan := findTargetSpan(spans, "proxy.target/Testdata.TestService/TestUnary", "foo:123")
	if targetSpan == nil {
		t.Fatal("target span not found")
	}

	if !spanHasEvent(targetSpan, "stream.connected") {
		t.Error("target span missing stream.connected event")
	}
	if !spanHasEvent(targetSpan, "stream.first_response") {
		t.Error("target span missing stream.first_response event")
	}
	if !spanHasEvent(targetSpan, "stream.finished") {
		t.Error("target span missing stream.finished event")
	}
	if !eventHasAttribute(targetSpan, "stream.finished", "grpc.status_code", "OK") {
		t.Error("stream.finished event missing grpc.status_code=OK")
	}
	if !spanHasEvent(targetSpan, "authz.evaluated") {
		t.Error("target span missing authz.evaluated event")
	}
	if !eventHasAttribute(targetSpan, "authz.evaluated", "sansshell.authz.result", "allowed") {
		t.Error("authz.evaluated event missing sansshell.authz.result=allowed")
	}
}

func TestTracing_AuthzDeniedEvent(t *testing.T) {
	exporter, _ := setupTracing(t)
	ctx := context.Background()

	policy := `
package sansshell.authz

default allow = false

allow {
  input.method = "/Testdata.TestService/TestUnary"
  input.message.input = "allowed"
}
`
	authz := testutil.NewOpaRPCAuthorizer(ctx, t, policy)
	testServerMap := testutil.StartTestDataServers(t, "foo:123")

	runTargetStream(t, testServerMap, "foo:123", "/Testdata.TestService/TestUnary", authz, &tdpb.TestRequest{Input: "denied_input"})

	spans := exporter.GetSpans()
	targetSpan := findTargetSpan(spans, "proxy.target/Testdata.TestService/TestUnary", "foo:123")
	if targetSpan == nil {
		t.Fatal("target span not found")
	}
	if !eventHasAttribute(targetSpan, "authz.evaluated", "sansshell.authz.result", "denied") {
		t.Error("expected authz.evaluated event with result=denied")
	}
}

func TestTracing_FanOutSpans(t *testing.T) {
	exporter, _ := setupTracing(t)
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:456")
	authz := testutil.NewAllowAllRPCAuthorizer(context.Background(), t)

	runTargetStream(t, testServerMap, "foo:123", "/Testdata.TestService/TestUnary", authz, &tdpb.TestRequest{Input: "a"})
	runTargetStream(t, testServerMap, "bar:456", "/Testdata.TestService/TestUnary", authz, &tdpb.TestRequest{Input: "b"})

	spans := exporter.GetSpans()

	expectedTargets := map[string]bool{"foo:123": false, "bar:456": false}
	for _, s := range spans {
		if s.Name == "proxy.target/Testdata.TestService/TestUnary" {
			for _, attr := range s.Attributes {
				if string(attr.Key) == "sansshell.target.address" {
					if _, ok := expectedTargets[attr.Value.AsString()]; ok {
						expectedTargets[attr.Value.AsString()] = true
					}
				}
			}
		}
	}
	for target, found := range expectedTargets {
		if !found {
			t.Errorf("expected target span for %s not found", target)
		}
	}

	if findTargetSpan(spans, "proxy.target.dial", "foo:123") == nil {
		t.Error("expected dial span for foo:123 not found")
	}
	if findTargetSpan(spans, "proxy.target.dial", "bar:456") == nil {
		t.Error("expected dial span for bar:456 not found")
	}
}

func TestTracing_EnrichRootSpan(t *testing.T) {
	exporter, tp := setupTracing(t)

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-root")

	peerInfo := &rpcauth.PeerAuthInput{
		Net:       &rpcauth.NetAuthInput{Network: "tcp", Address: "10.0.0.1"},
		Principal: &rpcauth.PrincipalAuthInput{ID: "user@example.com"},
	}
	md := metadata.Pairs(rpcauth.ReqJustKey, "ticket-123")
	ctx = metadata.NewIncomingContext(ctx, md)

	enrichRootSpan(ctx, peerInfo)
	span.End()

	spans := exporter.GetSpans()
	rootSpan := findSpan(spans, "test-root")
	if rootSpan == nil {
		t.Fatal("root span not found")
	}
	if !spanHasAttribute(rootSpan, "sansshell.caller.principal", "user@example.com") {
		t.Error("missing sansshell.caller.principal")
	}
	if !spanHasAttribute(rootSpan, "sansshell.caller.address", "10.0.0.1") {
		t.Error("missing sansshell.caller.address")
	}
	if !spanHasAttribute(rootSpan, "sansshell.proxy.justification", "ticket-123") {
		t.Error("missing sansshell.proxy.justification")
	}
}
