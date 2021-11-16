// package telemetry contains code for emitting telemetry
// from Sansshell processes.
package telemetry

import (
	"context"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// UnaryServerLogInterceptor returns a new gprc.UnaryServerInterceptor that logs
// incoming requests using the supplied logger, as well as injecting it into the
// context of downstream handlers.
func UnaryServerLogInterceptor(logger logr.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		l := logger.WithValues("method", info.FullMethod)
		if p, ok := peer.FromContext(ctx); ok {
			l = l.WithValues("peer-address", p.Addr)
		}
		l.Info("new request")
		logCtx := logr.NewContext(ctx, l)
		resp, err := handler(logCtx, req)
		if err != nil {
			l.Error(err, "")
		}
		return resp, err
	}
}

// StreamServerLogInterceptor returns a new grpc.StreamServerInterceptor that logs
// incoming streams using the supplied logger, and makes it available via the stream
// context to stream handlers.
func StreamServerLogInterceptor(logger logr.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		l := logger.WithValues("method", info.FullMethod)
		if p, ok := peer.FromContext(ss.Context()); ok {
			l = l.WithValues("peer-address", p.Addr)
		}
		l.Info("new stream")
		stream := &loggedStream{
			ServerStream: ss,
			logger:       l,
			logCtx:       logr.NewContext(ss.Context(), l),
		}
		return handler(srv, stream)
	}
}

// loggedStream wraps a grpc.ServerStream with additional logging.
type loggedStream struct {
	grpc.ServerStream
	logger logr.Logger
	logCtx context.Context
}

func (l *loggedStream) Context() context.Context {
	return l.logCtx
}

func (l *loggedStream) SendMsg(m interface{}) error {
	l.logger.V(1).Info("SendMsg")
	err := l.ServerStream.SendMsg(m)
	if err != nil {
		l.logger.Error(err, "")
	}
	return err
}

func (l *loggedStream) RecvMsg(m interface{}) error {
	l.logger.V(1).Info("RecvMsg")
	err := l.ServerStream.RecvMsg(m)
	if err != nil {
		l.logger.Error(err, "")
	}
	return err
}
