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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// MetricsRecorder contains methods used for collecting metrics.
type MetricsRecorder interface {
	// Counter increments the counter
	Counter(ctx context.Context, metric MetricDefinition, value int64, attributes ...attribute.KeyValue) error
	// CounterOrLog calls Counter, and log the error if any instead of returning it
	CounterOrLog(ctx context.Context, metric MetricDefinition, value int64, attributes ...attribute.KeyValue)
	// Gauge registers a gauge of int64 type and a callback function for updating the gauge value
	Gauge(ctx context.Context, metric MetricDefinition, callback instrument.Int64Callback, attributes ...attribute.KeyValue) error
	// CounterOrLog calls Gauge, and log the error if any instead of returning it
	GaugeOrLog(ctx context.Context, metric MetricDefinition, callback instrument.Int64Callback, attributes ...attribute.KeyValue)
}

// MetricDefinition specifies the metric name and description
type MetricDefinition struct {
	Name        string
	Description string
}

// contextKey is how we find MetricsRecorder in a context.Context.
type contextKey struct{}

// NewContextWithRecorder returns context with recorder attached.
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
	recorder := RecorderFromContext(ctx)
	if recorder != nil {
		return recorder
	}

	return noOpRecorder{}
}

// noOpRecorder is a no-op implementation of MetricsRecorder.
type noOpRecorder struct {
}

func (n noOpRecorder) Counter(ctx context.Context, metric MetricDefinition, value int64, attributes ...attribute.KeyValue) error {
	return nil
}
func (n noOpRecorder) CounterOrLog(ctx context.Context, metric MetricDefinition, value int64, attributes ...attribute.KeyValue) {
}
func (n noOpRecorder) Gauge(ctx context.Context, metric MetricDefinition, callback instrument.Int64Callback, attributes ...attribute.KeyValue) error {
	return nil
}
func (n noOpRecorder) GaugeOrLog(ctx context.Context, metric MetricDefinition, callback instrument.Int64Callback, attributes ...attribute.KeyValue) {
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

// Context returns the client stream context with the MetricsRecorder attached
func (o *observedClientStream) Context() context.Context {
	ctx := o.ClientStream.Context()
	ctx = NewContextWithRecorder(ctx, o.metricsRecorder)
	return ctx
}

// UnaryServerMetricsInterceptor returns an unary server grpc interceptor which adds recorder to the grpc request context
func UnaryServerMetricsInterceptor(recorder MetricsRecorder) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		recorderCtx := NewContextWithRecorder(ctx, recorder)

		peer, ok := peer.FromContext(ctx)
		if ok {
			tlsInfo := peer.AuthInfo.(credentials.TLSInfo)
			state := tlsInfo.State
			if state.HandshakeComplete && !state.DidResume {
				recorder.Counter(ctx, MetricDefinition{Name: "tls_unary_ok", Description: "tls ok"}, 1)
			} else {
				recorder.Counter(ctx, MetricDefinition{Name: "tls_unary_failed", Description: "tls failed"}, 1)
			}
		}
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
		peer, ok := peer.FromContext(ss.Context())
		if ok {
			tlsInfo := peer.AuthInfo.(credentials.TLSInfo)
			state := tlsInfo.State
			if state.HandshakeComplete && !state.DidResume {
				recorder.Counter(ss.Context(), MetricDefinition{Name: "tls_ok", Description: "tls ok"}, 1)
			} else {
				recorder.Counter(ss.Context(), MetricDefinition{Name: "tls_failed", Description: "tls failed"}, 1)
			}
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

// Context returns the server stream context attached with a MetricsRecorder
func (o *observedStream) Context() context.Context {
	// Get the stream context and make sure our recorder is attached.
	ctx := o.ServerStream.Context()
	ctx = NewContextWithRecorder(ctx, o.metricsrecorder)
	return ctx
}
