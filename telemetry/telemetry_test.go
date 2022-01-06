package telemetry

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

// Need a null clientStream for testing.
type clientStream struct{}

func (*clientStream) Header() (metadata.MD, error) {
	return nil, errors.New("unimplemented")
}

func (*clientStream) Trailer() metadata.MD {
	return nil
}

func (*clientStream) CloseSend() error {
	return errors.New("unimplemented")
}

func (*clientStream) Context() context.Context {
	return context.Background()
}

func (*clientStream) SendMsg(interface{}) error {
	return errors.New("unimplemented")
}

func (*clientStream) RecvMsg(interface{}) error {
	return errors.New("unimplemented")
}

func TestUnaryClient(t *testing.T) {
	// We need the basics of a connection to satisfy a real ClientConn below.
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("Failed to dial bufnet", err, t)

	var args string
	fn := func(p, a string) {
		args = a
	}
	logger := funcr.New(fn, funcr.Options{})

	intercept := UnaryClientLogInterceptor(logger)

	wantMethod := "foo"
	wantError := "error"
	// Testing is a little weird. This will be called below when we call intercept. Then additional state
	// gets set on the error return we test below that.
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		// Validate this is a proper logger context.
		if _, err := logr.FromContext(ctx); err != nil {
			t.Fatal("didn't get passed a logging context")
		}
		if got, want := method, wantMethod; got != want {
			t.Fatalf("didn't get expected method. got %s want %s", got, want)
		}
		// The logging should have happened by now
		want := "new client request"
		if !strings.Contains(args, want) {
			t.Fatalf("didn't get expected logging args. got %q want %q within it", args, want)
		}
		// Return an error
		return errors.New(wantError)
	}

	err = intercept(context.Background(), wantMethod, nil, nil, conn, invoker)
	t.Log(err)
	if err == nil {
		t.Fatal("didn't get expected error from intercept")
	}
	if got, want := err.Error(), wantError; got != want {
		t.Fatalf("didn't get expected error. got %v want %v", got, want)
	}
}

func TestStreamClient(t *testing.T) {
	// We need the basics of a connection to satisfy a real ClientConn below.
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("Failed to dial bufnet", err, t)

	var args string
	fn := func(p, a string) {
		args = a
	}
	logger := funcr.New(fn, funcr.Options{})

	intercept := StreamClientLogInterceptor(logger)

	wantMethod := "foo"
	wantError := "error"
	// Testing is a little weird. This will be called below when we call intercept. Then additional state
	// gets set on the error return we test below that.
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// Validate this is a proper logger context.
		if _, err := logr.FromContext(ctx); err != nil {
			t.Fatal("didn't get passed a logging context")
		}
		if got, want := method, wantMethod; got != want {
			t.Fatalf("didn't get expected method. got %s want %s", got, want)
		}
		// The logging should have happened by now
		want := "new client stream"
		if !strings.Contains(args, want) {
			t.Fatalf("didn't get expected logging args. got %q want %q within it", args, want)
		}
		// Return an error
		if method == "foo" {
			return nil, errors.New(wantError)
		}
		return &clientStream{}, nil
	}
	stream, err := intercept(context.Background(), nil, conn, wantMethod, streamer)
	t.Log(err)
	if err == nil {
		t.Fatal("didn't get expected error from streamer")
	}
	if got, want := err.Error(), wantError; got != want {
		t.Fatalf("didn't get expected error. got %v want %v", got, want)
	}
	if stream != nil {
		t.Fatal("got stream back even with an error?")
	}

	// Shouldn't get an error now and we get a real stream.
	wantMethod = "bar"
	stream, err = intercept(context.Background(), nil, conn, wantMethod, streamer)
	if err != nil {
		t.Fatalf("unexpected error from 2nd streamer call: %v", err)
	}

	if _, err := logr.FromContext(stream.Context()); err != nil {
		t.Fatal("returned stream doesn't contain a logging context")
	}
}
