package testutil

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"google.golang.org/grpc/metadata"
)

// ResolvePath takes a binary name and attempts to resolve it for testing or fatal out.
func ResolvePath(t *testing.T, path string) string {
	t.Helper()
	binPath, err := exec.LookPath(path)
	if err != nil {
		t.Fatalf("Can't find path for %s: %v", path, err)
	}
	return binPath
}

// FataOnErr is a testing helper function to test and abort on an error.
// Reduces 3 lines to 1 for common error checking.
func FatalOnErr(op string, e error, t *testing.T) {
	t.Helper()
	if e != nil {
		t.Fatalf("%s: err was %v, want nil", op, e)
	}
}

// Need a null ClientStream for testing.
type FakeClientStream struct{}

// See grpc.ClientStream
func (*FakeClientStream) Header() (metadata.MD, error) {
	return nil, errors.New("unimplemented")
}

// See grpc.ClientStream
func (*FakeClientStream) Trailer() metadata.MD {
	return nil
}

// See grpc.ClientStream
func (*FakeClientStream) CloseSend() error {
	return errors.New("unimplemented")
}

// See grpc.ClientStream
func (*FakeClientStream) Context() context.Context {
	return context.Background()
}

// See grpc.ClientStream
func (*FakeClientStream) SendMsg(interface{}) error {
	return errors.New("unimplemented")
}

// See grpc.ClientStream
func (*FakeClientStream) RecvMsg(interface{}) error {
	return errors.New("unimplemented")
}

// Need a null ServerStream for testing
type FakeServerStream struct {
	Ctx context.Context
}

// See grpc.ServerStream
func (*FakeServerStream) SetHeader(metadata.MD) error {
	return errors.New("unimplemented")
}

// See grpc.ServerStream
func (*FakeServerStream) SendHeader(metadata.MD) error {
	return errors.New("unimplemented")
}

// See grpc.ServerStream
func (*FakeServerStream) SetTrailer(metadata.MD) {
}

// See grpc.ServerStream
func (f *FakeServerStream) Context() context.Context {
	return f.Ctx
}

// See grpc.ServerStream
func (*FakeServerStream) SendMsg(interface{}) error {
	return errors.New("unimplemented")
}

// See grpc.ServerStream
func (*FakeServerStream) RecvMsg(interface{}) error {
	return errors.New("unimplemented")
}
