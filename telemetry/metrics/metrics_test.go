/*
Copyright (c) 2023 Snowflake Inc. All rights reserved.

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
package metrics

import (
	"context"
	"net"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/metric/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestNewContextWithRecorder(t *testing.T) {
	want := noOpRecorder{}
	ctx := context.Background()
	ctx = NewContextWithRecorder(ctx, want)
	got := ctx.Value(contextKey{})
	// Asserts that recorder is in the context
	if got != want {
		t.Errorf("NewContextWithRecorder(ctx, want).Value(contextKey{})=%v, want %v", got, want)
	}
}

func TestRecorderFromContext(t *testing.T) {
	ctx := context.Background()
	got := RecorderFromContext(ctx)
	// Asserts that RecorderFromContext returns nil if there's no recorder in the context
	if got != nil {
		t.Errorf("Expected nil when no recorder is in context, but got %v", got)
	}

	want := noOpRecorder{}
	ctx = NewContextWithRecorder(ctx, want)
	got = RecorderFromContext(ctx)
	if got != want {
		t.Errorf("Expected recorder is in context, but got: %v", got)
	}
}

func TestRecorderFromContextOrNoop(t *testing.T) {
	ctx := context.Background()
	got := RecorderFromContextOrNoop(ctx)
	// Asserts that RecorderFromContextOrNoop returns noOpRecorder if there's no recorder in the context
	noop := noOpRecorder{}
	if got != noop {
		t.Errorf("Expected %v when no recorder is in context, but got %v", noop, got)
	}

	want, err := NewOtelRecorder(global.Meter("test"))
	if err != nil {
		testutil.FatalOnErr("create new otel recorder", err, t)
	}
	ctx = NewContextWithRecorder(ctx, want)
	got = RecorderFromContextOrNoop(ctx)
	// Asserts that RecorderFromContextOrNoop
	if got != want {
		t.Errorf("Expected RecorderFromContextOrNoop to returns %v, but got: %v", want, got)
	}
}

var (
	bufSize = 1024 * 1024
	lis     = bufconn.Listen(bufSize)
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestUnaryClientInterceptor(t *testing.T) {
	// We need the basics of a connection to satisfy a real ClientConn below.
	bgCtx := context.Background()
	conn, err := grpc.DialContext(bgCtx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)

	recorder, err := NewOtelRecorder(global.Meter("test"))
	if err != nil {
		testutil.FatalOnErr("create new otel recorder", err, t)
	}
	interceptor := UnaryClientMetricsInterceptor(recorder)
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		// Validate that context contains recorder
		if got := RecorderFromContext(ctx); got != recorder {
			t.Fatal("didn't get passed a metrics recorder context")
		}
		return nil
	}
	err = interceptor(bgCtx, "", nil, nil, conn, invoker)
	testutil.FatalOnErr("didn't expect an error", err, t)
}

func TestStreamClientInterceptor(t *testing.T) {
	// We need the basics of a connection to satisfy a real ClientConn below.
	bgCtx := context.Background()
	conn, err := grpc.DialContext(bgCtx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)

	recorder, err := NewOtelRecorder(global.Meter("test"))
	if err != nil {
		testutil.FatalOnErr("create new otel recorder", err, t)
	}

	interceptor := StreamClientMetricsInterceptor(recorder)
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// Validate that context contains recorder
		if got := RecorderFromContext(ctx); got != recorder {
			t.Fatal("didn't get passed a metrics recorder context")
		}
		return &testutil.FakeClientStream{}, nil
	}
	stream, err := interceptor(bgCtx, nil, conn, "", streamer)
	testutil.FatalOnErr("didn't expect an error", err, t)
	if stream == nil {
		t.Fatal("got a nil stream with no error?")
	}

	streamerWithErr := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return &testutil.FakeClientStream{}, errors.New("expectederr")
	}
	stream, err = interceptor(bgCtx, nil, conn, "", streamerWithErr)
	testutil.FatalOnNoErr("expected an error but got %v", err, t)
	if stream != nil {
		t.Fatalf("expected interceptor to returns nil on error, but got %v", stream)
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	bgCtx := context.Background()
	recorder, err := NewOtelRecorder(global.Meter("test"))
	if err != nil {
		testutil.FatalOnErr("create new otel recorder", err, t)
	}

	interceptor := UnaryServerMetricsInterceptor(recorder)
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Validate that context contains recorder
		if got := RecorderFromContext(ctx); got != recorder {
			t.Fatal("didn't get passed a metrics recorder context")
		}
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{
		FullMethod: "fullmethod",
	}
	_, err = interceptor(bgCtx, nil, info, handler)
	testutil.FatalOnErr("didn't expect an error", err, t)
}

func TestStreamServerInterceptor(t *testing.T) {
	ctx := context.Background()
	recorder, err := NewOtelRecorder(global.Meter("test"))
	if err != nil {
		testutil.FatalOnErr("create new otel recorder", err, t)
	}

	interceptor := StreamServerMetricsInterceptor(recorder)
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		// Validate that context contains recorder
		if got := RecorderFromContext(stream.Context()); got != recorder {
			t.Fatal("didn't get passed a metrics recorder context")
		}
		return nil
	}
	info := &grpc.StreamServerInfo{
		FullMethod: "fullmethod",
	}
	ss := &testutil.FakeServerStream{
		Ctx: ctx,
	}
	err = interceptor(nil, ss, info, handler)
	testutil.FatalOnErr("didn't expect an error", err, t)
}
