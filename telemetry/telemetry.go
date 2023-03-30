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

// Package telemetry contains code for emitting telemetry
// from Sansshell processes.
package telemetry

import (
	"context"
	"io"
	"strings"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	sansshellMetadata   = "sansshell-"
	sansshellTraceIDKey = sansshellMetadata + "trace-id"
)

// UnaryClientLogInterceptor returns a new grpc.UnaryClientInterceptor that logs
// outgoing requests using the supplied logger, as well as injecting it into the
// context of the invoker.
func UnaryClientLogInterceptor(logger logr.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		l := logger.WithValues("method", method, "target", cc.Target())
		logCtx := logr.NewContext(ctx, l)
		logCtx = passAlongMetadata(logCtx)
		l = logMetadata(logCtx, l)
		l.Info("new client request")
		err := invoker(logCtx, method, req, reply, cc, opts...)
		if err != nil {
			l.Error(err, "")
		}
		return err
	}
}

// StreamClientLogInterceptor returns a new grpc.StreamClientInterceptor that logs
// client requests using the supplied logger, as well as injecting it into the context
// of the created stream.
func StreamClientLogInterceptor(logger logr.Logger) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		l := logger.WithValues("method", method, "target", cc.Target())
		logCtx := logr.NewContext(ctx, l)
		logCtx = passAlongMetadata(logCtx)
		l = logMetadata(logCtx, l)
		l.Info("new client stream")
		stream, err := streamer(logCtx, desc, cc, method, opts...)
		if err != nil {
			l.Error(err, "create stream")
			return nil, err
		}
		return &loggedClientStream{
			ClientStream: stream,
			logger:       l,
		}, nil
	}
}

func hasSpan(ctx context.Context) bool {
	return trace.SpanContextFromContext(ctx).IsValid()
}

// Add trace ID to logger if there's an active span
func logOtelTraceID(ctx context.Context, l logr.Logger) logr.Logger {
	if hasSpan(ctx) {
		spanCtx := trace.SpanContextFromContext(ctx)
		l = l.WithValues(sansshellTraceIDKey, spanCtx.TraceID().String())
	}

	return l
}

func logMetadata(ctx context.Context, l logr.Logger) logr.Logger {
	// Add any sansshell specific metadata from incoming context to the logging we do.
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		for k, v := range md {
			if strings.HasPrefix(k, sansshellMetadata) {
				for _, val := range v {
					l = l.WithValues(k, val)
				}
			}
		}
	}
	l = logOtelTraceID(ctx, l)
	return l
}

func passAlongMetadata(ctx context.Context) context.Context {
	// See if we got any metadata that has our prefix and pass it along
	// downstream (i.e. proxy case).
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		for k, v := range md {
			if strings.HasPrefix(k, sansshellMetadata) {
				for _, val := range v {
					ctx = metadata.AppendToOutgoingContext(ctx, k, val)
				}
			}
		}
	}
	return ctx
}

type loggedClientStream struct {
	grpc.ClientStream
	logger logr.Logger
}

// See: grpc.ClientStream.Context()
func (l *loggedClientStream) Context() context.Context {
	// Get the stream context and make sure our logger is attached.
	ctx := l.ClientStream.Context()
	ctx = logr.NewContext(ctx, l.logger)
	return ctx
}

// See: grpc.ClientStream.SendMsg()
func (l *loggedClientStream) SendMsg(m interface{}) error {
	l.logger.V(1).Info("SendMsg")
	err := l.ClientStream.SendMsg(m)
	if err != nil {
		l.logger.Error(err, "SendMsg")
	}
	return err
}

// See: grpc.ClientStream.RecvMsg()
func (l *loggedClientStream) RecvMsg(m interface{}) error {
	l.logger.V(1).Info("RecvMsg")
	err := l.ClientStream.RecvMsg(m)
	if err != nil && err != io.EOF {
		l.logger.Error(err, "RecvMsg")
	}
	return err
}

// See: grpc.ClientStream.CloseSend()
func (l *loggedClientStream) CloseSend() error {
	l.logger.Info("CloseSend")
	err := l.ClientStream.CloseSend()
	if err != nil {
		l.logger.Error(err, "CloseSend")
	}
	return err
}

// UnaryServerLogInterceptor returns a new gprc.UnaryServerInterceptor that logs
// incoming requests using the supplied logger, as well as injecting it into the
// context of downstream handlers. If incoming calls require client side provided justification
// (which is logged) then the justification parameter should be true and a required
// key of ReqJustKey must be in the context when the interceptor runs.
func UnaryServerLogInterceptor(logger logr.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		l := logger.WithValues("method", info.FullMethod)
		p := rpcauth.PeerInputFromContext(ctx)
		if p != nil {
			l = l.WithValues("peer", p)
		}
		l = logMetadata(ctx, l)
		l.Info("new request")
		logCtx := logr.NewContext(ctx, l)
		resp, err := handler(logCtx, req)
		if err != nil {
			l.Error(err, "handler")
		}
		return resp, err
	}
}

// StreamServerLogInterceptor returns a new grpc.StreamServerInterceptor that logs
// incoming streams using the supplied logger, and makes it available via the stream
// context to stream handlers. If incoming calls require client side provided justification
// (which is logged) then the justification parameter should be true and a required
// key of ReqJustKey must be in the context when the interceptor runs.
func StreamServerLogInterceptor(logger logr.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		l := logger.WithValues("method", info.FullMethod)
		p := rpcauth.PeerInputFromContext(ss.Context())
		if p != nil {
			l = l.WithValues("peer", p)
		}
		l = logMetadata(ss.Context(), l)
		l.Info("new stream")
		stream := &loggedStream{
			ServerStream: ss,
			logger:       l,
		}
		err := handler(srv, stream)
		if err != nil {
			l.Error(err, "handler")
		}
		return err
	}
}

// loggedStream wraps a grpc.ServerStream with additional logging.
type loggedStream struct {
	grpc.ServerStream
	logger logr.Logger
}

func (l *loggedStream) Context() context.Context {
	// Get the stream context and make sure our logger is attached.
	ctx := l.ServerStream.Context()
	ctx = logr.NewContext(ctx, l.logger)
	return ctx
}

func (l *loggedStream) SendMsg(m interface{}) error {
	l.logger.V(1).Info("SendMsg")
	err := l.ServerStream.SendMsg(m)
	if err != nil {
		l.logger.Error(err, "SendMsg")
	}
	return err
}

func (l *loggedStream) RecvMsg(m interface{}) error {
	l.logger.V(1).Info("RecvMsg")
	err := l.ServerStream.RecvMsg(m)
	if err != nil && err != io.EOF {
		l.logger.Error(err, "RecvMsg")
	}
	return err
}
