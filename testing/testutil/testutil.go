package testutil

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/testing/protocmp"
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

// WantErr is a testing helper for comparing boolean expected error state.
// Reduces 3 lines to 1 for common error checking.
func WantErr(err error, want bool, t *testing.T) {
	t.Helper()
	if got := err != nil; got != want {
		t.Fatalf("unexpected error state. got %t want %t err %v", got, want, err)
	}
}

// DiffErr compares 2 messages (including optional transforms) and throws
// a testing Fatal on diff. This assumes they are usually proto messages so will
// automatically include protocmp.Transform() for the caller.
// Reduces 3 lines to 1 for common error checking.
func DiffErr(resp interface{}, compare interface{}, t *testing.T, opts ...cmp.Option) {
	t.Helper()
	diffOpts := []cmp.Option{protocmp.Transform()}
	diffOpts = append(diffOpts, opts...)
	if diff := cmp.Diff(resp, compare, diffOpts...); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, compare, diff)
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
