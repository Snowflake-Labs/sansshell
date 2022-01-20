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
	"google.golang.org/grpc/peer"
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

func testLogging(t *testing.T, args, want string) {
	t.Helper()
	if !strings.Contains(args, want) {
		t.Fatalf("didn't get expected logging args. got %q want %q within it", args, want)
	}
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
		testLogging(t, args, "new client request")
		// Return an error
		return errors.New(wantError)
	}

	err = intercept(context.Background(), wantMethod, nil, nil, conn, invoker)
	t.Log(err)
	testutil.FatalOnNoErr("intercept", err, t)
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

	wantMethod := "sendError"
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
		testLogging(t, args, "new client stream")
		// Return an error
		if method == "sendError" {
			return nil, errors.New(wantError)
		}
		return &testutil.FakeClientStream{}, nil
	}
	stream, err := intercept(context.Background(), nil, conn, wantMethod, streamer)
	t.Log(err)
	testutil.FatalOnNoErr("streamer", err, t)
	if got, want := err.Error(), wantError; got != want {
		t.Fatalf("didn't get expected error. got %v want %v", got, want)
	}
	if stream != nil {
		t.Fatal("got stream back even with an error?")
	}

	// Shouldn't get an error now and we get a real stream.
	wantMethod = "bar"
	stream, err = intercept(context.Background(), nil, conn, wantMethod, streamer)
	testutil.FatalOnErr("2nd streamer call", err, t)

	if _, err := logr.FromContext(stream.Context()); err != nil {
		t.Fatal("returned stream doesn't contain a logging context")
	}

	if err := stream.SendMsg(nil); err == nil {
		t.Fatal("didn't get error from SendMsg on fake client stream")
	}

	// The error logging should have happened by now.
	testLogging(t, args, "SendMsg")

	err = stream.RecvMsg(nil)
	testutil.FatalOnNoErr("RecvMsg on fake", err, t)

	// The error logging should have happened by now.
	testLogging(t, args, "RecvMsg")

	err = stream.CloseSend()
	testutil.FatalOnNoErr("CloseSend on fake", err, t)

	// The error logging should have happened by now.
	testLogging(t, args, "CloseSend")
}

func TestUnaryServer(t *testing.T) {
	var args string
	fn := func(p, a string) {
		args = a
	}
	logger := funcr.New(fn, funcr.Options{})

	intercept := UnaryServerLogInterceptor(logger)

	wantMethod := "foo"
	wantError := "error"
	// Testing is a little weird. This will be called below when we call intercept. Then additional state
	// gets set on the error return we test below that.
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Validate this is a proper logger context.
		if _, err := logr.FromContext(ctx); err != nil {
			t.Fatal("didn't get passed a logging context")
		}

		// The logging should have happened by now
		testLogging(t, args, wantMethod)
		testLogging(t, args, "new request")
		// Return an error
		return nil, errors.New(wantError)
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: wantMethod,
	}
	ctx := peer.NewContext(context.Background(), &peer.Peer{})
	_, err := intercept(ctx, nil, info, handler)
	t.Log(err)
	testutil.FatalOnNoErr("intercept", err, t)
	if got, want := err.Error(), wantError; got != want {
		t.Fatalf("didn't get expected error. got %v want %v", got, want)
	}
}

func TestStreamServer(t *testing.T) {
	var args string
	fn := func(p, a string) {
		args = a
	}
	logger := funcr.New(fn, funcr.Options{})

	intercept := StreamServerLogInterceptor(logger)

	wantMethod := "foo"
	wantError := "error"
	// Testing is a little weird. This will be called below when we call intercept. Then additional state
	// gets set on the error return we test below that.
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		// Validate this is a proper logger context.
		if _, err := logr.FromContext(stream.Context()); err != nil {
			t.Fatal("didn't get passed a logging context")
		}

		// The logging should have happened by now
		testLogging(t, args, wantMethod)
		testLogging(t, args, "new stream")

		if err := stream.SendMsg(nil); err == nil {
			t.Fatal("didn't get error from SendMsg on fake client stream")
		}

		// The error logging should have happened by now.
		testLogging(t, args, "SendMsg")

		if err := stream.RecvMsg(nil); err == nil {
			t.Fatal("didn't get error from RecvMsg on fake client stream")
		}
		// The error logging should have happened by now.
		testLogging(t, args, "RecvMsg")

		// Return an error
		return errors.New(wantError)
	}

	info := &grpc.StreamServerInfo{
		FullMethod: wantMethod,
	}
	ctx := peer.NewContext(context.Background(), &peer.Peer{})
	ss := &testutil.FakeServerStream{
		Ctx: ctx,
	}

	err := intercept(nil, ss, info, handler)
	t.Log(err)
	testutil.FatalOnNoErr("intercept", err, t)
	if got, want := err.Error(), wantError; got != want {
		t.Fatalf("didn't get expected error. got %v want %v", got, want)
	}
}
