package telemetry

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type sansshelContextKey string

func (c sansshelContextKey) String() string {
	return string(c)
}

const (
	sansshellTraceIDKey        = sansshellMetadata + "trace-id"
	sansshellTraceIDContextKey = sansshelContextKey(sansshellTraceIDKey)
)

func UnaryServerTraceInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Get span context and attach trace ID to both current and outgoing context
		spanctx := trace.SpanContextFromContext(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx, sansshellTraceIDKey, spanctx.TraceID().String())
		resp, err := handler(ctx, req)
		if err != nil {
			l := logr.FromContextOrDiscard(ctx)
			l.Error(err, "handler")
		}
		return resp, err
	}
}

func StreamServerTraceInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		stream := &tracedStream{
			ServerStream: ss,
		}
		err := handler(srv, stream)
		if err != nil {
			l := logr.FromContextOrDiscard(ss.Context())
			l.Error(err, "handler")
		}
		return err
	}
}

type tracedStream struct {
	grpc.ServerStream
}

func (t *tracedStream) Context() context.Context {
	// Get the span context and attach it to both current and outgoing context
	ctx := t.ServerStream.Context()
	spanctx := trace.SpanContextFromContext(ctx)
	ctx = metadata.AppendToOutgoingContext(ctx, sansshellTraceIDKey, spanctx.TraceID().String())
	return ctx
}
