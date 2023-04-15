/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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
// package metrics contains code for adding
// metric instrumentations to Sansshell processes
package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"google.golang.org/grpc"
)

type MetricsRecorder interface {
	RegisterInt64Counter(name, description string) error
	RegisterInt64Gauge(name, description string, callback instrument.Int64Callback, attributes ...attribute.KeyValue) error
	AddInt64Counter(ctx context.Context, name string, value int64, attributes ...attribute.KeyValue) error
}

// contextKey is how we find Metrics in a context.Context.
type contextKey struct{}

func NewContextWithRecorder(ctx context.Context, recorder MetricsRecorder) context.Context {
	return context.WithValue(ctx, contextKey{}, recorder)
}

func RecorderFromContext(ctx context.Context) MetricsRecorder {
	if v, ok := ctx.Value(contextKey{}).(MetricsRecorder); ok {
		return v
	}

	return nil
}

func RecorderFromContextOrNoop(ctx context.Context) MetricsRecorder {
	mr := RecorderFromContext(ctx)
	if mr != nil {
		return mr
	}

	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("NOOP")

	return noOpRecorder{}
}

// noOpRecorder is a no-op implementation of MetricsRecorder.
type noOpRecorder struct {
}

func (n noOpRecorder) RegisterInt64Counter(name, description string) error {
	return nil
}
func (n noOpRecorder) RegisterInt64Gauge(name, description string, callback instrument.Int64Callback, attributes ...attribute.KeyValue) error {
	return nil
}
func (n noOpRecorder) AddInt64Counter(ctx context.Context, name string, value int64, attributes ...attribute.KeyValue) error {
	return nil
}

func UnaryClientMetricsInterceptor(recorder MetricsRecorder) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		recorderCtx := NewContextWithRecorder(ctx, recorder)
		return invoker(recorderCtx, method, req, reply, cc, opts...)
	}
}

func StreamClientMetricsInterceptor(recorder MetricsRecorder) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		recorderCtx := NewContextWithRecorder(ctx, recorder)
		stream, err := streamer(recorderCtx, desc, cc, method, opts...)
		if err != nil {
			return nil, err
		}
		return &observedClientStream{
			ClientStream:    stream,
			metricsRecorder: recorder,
		}, nil
	}
}

type observedClientStream struct {
	grpc.ClientStream
	metricsRecorder MetricsRecorder
}

func (o *observedClientStream) Context() context.Context {
	ctx := o.ClientStream.Context()
	ctx = NewContextWithRecorder(ctx, o.metricsRecorder)
	return ctx
}

func UnaryServerMetricsInterceptor(recorder MetricsRecorder) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		recorderCtx := NewContextWithRecorder(ctx, recorder)
		return handler(recorderCtx, req)
	}
}

func StreamServerMetricsInterceptor(recorder MetricsRecorder) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		stream := &observedStream{
			ServerStream:    ss,
			metricsrecorder: recorder,
		}
		err := handler(srv, stream)
		return err
	}
}

type observedStream struct {
	grpc.ServerStream
	metricsrecorder MetricsRecorder
}

func (o *observedStream) Context() context.Context {
	// Get the stream context and make sure our recorder is attached.
	ctx := o.ServerStream.Context()
	ctx = NewContextWithRecorder(ctx, o.metricsrecorder)
	return ctx
}
