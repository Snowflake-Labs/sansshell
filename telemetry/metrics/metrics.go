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

// MetricsRecorder contains methods used for collecting metrics.
type MetricsRecorder interface {
	// RegisterInt64Counter registers a counter of int64 type
	RegisterInt64Counter(name, description string) error
	// RegisterInt64Gauge registers a gauge of int64 type and a callback function for updating the gauge value
	RegisterInt64Gauge(name, description string, callback instrument.Int64Callback) error
	// AddInt64Counter increments the counter value
	AddInt64Counter(ctx context.Context, name string, value int64, attributes ...attribute.KeyValue) error
}

// contextKey is how we find Metrics in a context.Context.
type contextKey struct{}

// NewContextWithRecorder returns the context with embedded MetricsRecorder.
func NewContextWithRecorder(ctx context.Context, recorder MetricsRecorder) context.Context {
	return context.WithValue(ctx, contextKey{}, recorder)
}

// RecorderFromContext returns the MetricsRecorder object in the context if exists
// Otherwise it returns nil
func RecorderFromContext(ctx context.Context) MetricsRecorder {
	if v, ok := ctx.Value(contextKey{}).(MetricsRecorder); ok {
		return v
	}

	return nil
}

// RecorderFromContextOrNoop returns the MetricsRecorder object in the context if exists
// Otherwise it returns a noOpRecorder object
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
func (n noOpRecorder) RegisterInt64Gauge(name, description string, callback instrument.Int64Callback) error {
	return nil
}
func (n noOpRecorder) AddInt64Counter(ctx context.Context, name string, value int64, attributes ...attribute.KeyValue) error {
	return nil
}

// UnaryClientMetricsInterceptor returns an unary client grpc interceptor which adds recorder to the grpc request context
func UnaryClientMetricsInterceptor(recorder MetricsRecorder) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		recorderCtx := NewContextWithRecorder(ctx, recorder)
		return invoker(recorderCtx, method, req, reply, cc, opts...)
	}
}

// StreamClientMetricsInterceptor returns a stream client grpc interceptor which adds recorder to the grpc stream context
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

// observedClientStream is a grpc client stream with an attached MetricsRecorder
type observedClientStream struct {
	grpc.ClientStream
	metricsRecorder MetricsRecorder
}

// *observedClientStream.Context returns the client stream context attached with a MetricsRecorder
func (o *observedClientStream) Context() context.Context {
	ctx := o.ClientStream.Context()
	ctx = NewContextWithRecorder(ctx, o.metricsRecorder)
	return ctx
}

// UnaryServerMetricsInterceptor returns an unary server grpc interceptor which adds recorder to the grpc request context
func UnaryServerMetricsInterceptor(recorder MetricsRecorder) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		recorderCtx := NewContextWithRecorder(ctx, recorder)
		return handler(recorderCtx, req)
	}
}

// StreamServerMetricsInterceptor returns a stream server grpc interceptor which adds recorder to the grpc stream context
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

// observedStream is a grpc server stream with an attached MetricsRecorder
type observedStream struct {
	grpc.ServerStream
	metricsrecorder MetricsRecorder
}

// *observedStream.Context returns the server stream context attached with a MetricsRecorder
func (o *observedStream) Context() context.Context {
	// Get the stream context and make sure our recorder is attached.
	ctx := o.ServerStream.Context()
	ctx = NewContextWithRecorder(ctx, o.metricsrecorder)
	return ctx
}
