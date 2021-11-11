package server

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	pb "github.com/Snowflake-Labs/sansshell/proxy"
	_ "github.com/Snowflake-Labs/sansshell/proxy/testdata"
)

// A TargetDialer than returns an error for all Dials
type dialErrTargetDialer codes.Code

func (e dialErrTargetDialer) DialContext(ctx context.Context, target string) (grpc.ClientConnInterface, error) {
	return nil, status.Error(codes.Code(e), "")
}

// A grpc.ClientConnInterface that returns errors for all methods
type errClientConn codes.Code

func (e errClientConn) Invoke(context.Context, string, interface{}, interface{}, ...grpc.CallOption) error {
	return status.Error(codes.Code(e), "")
}

func (e errClientConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) error {
	return status.Error(codes.Code(e), "")
}

func TestEmptyStreamSet(t *testing.T) {
	ctx := context.Background()
	errDialer := dialErrTargetDialer(codes.Unimplemented)
	ss := NewTargetStreamSet(map[string]*ServiceMethod{}, errDialer, nil)

	// wait does not block when no work is being done
	finishedWait := make(chan struct{})
	go func() {
		select {
		case <-finishedWait:
			return
		case <-time.After(time.Second * 5):
			t.Fatal("TargetStreamSet.Wait() blocked on empty set, want no block")
		}
	}()
	ss.Wait()
	close(finishedWait)

	// Close of nonexistent ids is an error
	if err := ss.ClientClose(&pb.ClientClose{StreamIds: []uint64{1}}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("TargetStream.ClientClose(1) err code was %v, want code.InvalidArgument", status.Code(err))
	}

	// Cancel of nonexistent ids is an error
	if err := ss.ClientCancel(&pb.ClientCancel{StreamIds: []uint64{1}}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("TargetStream.ClientCancel(1) err code was %v, want code.InvalidArgument", status.Code(err))
	}

	// arbitrary payload
	payload, _ := anypb.New(&pb.StreamData{})

	// Send to nonexistent ids is an error
	if err := ss.Send(ctx, &pb.StreamData{StreamIds: []uint64{1}, Payload: payload}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("TargetStream.ClientCancel(0) err code was %v, want code.InvalidArgument", status.Code(err))
	}
}

func TestStreamSetAddErrors(t *testing.T) {
	errDialer := dialErrTargetDialer(codes.Unimplemented)
	serviceMap := LoadGlobalServiceMap()
	ss := NewTargetStreamSet(serviceMap, errDialer, nil)

	// buffered reply channel, so that Add will not block
	replyChan := make(chan *pb.ProxyReply, 1)

	for _, tc := range []struct {
		name    string
		method  string
		nonce   uint32
		errCode codes.Code
	}{
		{
			name:    "dial failure",
			nonce:   1,
			method:  "/Testdata.TestService/TestUnary",
			errCode: codes.Internal,
		},
		{
			name:    "method lookup failure",
			nonce:   2,
			method:  "/Nosuch.Method/Foo",
			errCode: codes.InvalidArgument,
		},
	} {
		req := &pb.StartStream{
			Target:     "nosuchhost:000",
			Nonce:      tc.nonce,
			MethodName: tc.method,
		}
		if err := ss.Add(context.Background(), req, replyChan, nil /*doneChan should not be called*/); err != nil {
			t.Fatalf("StartStream(%+v), err was %v, want nil", req, err)
		}
		var msg *pb.ProxyReply
		select {
		case msg = <-replyChan:
			// success
		case <-time.After(time.Second * 1):
			t.Fatalf("%s: TargetStreamSet.Add() reply not sent after 1 second", tc.name)
		}
		ssr := msg.GetStartStreamReply()
		if ssr == nil {
			t.Fatalf("%s: TargetStreamSet.Add() got unexpected reply type %T, want StartStreamReply", tc.name, msg.Reply)
		}
		if ssr.Nonce != tc.nonce {
			t.Errorf("%s: TargetStreamSet.Add() nonce was %v, want %v", tc.name, ssr.Nonce, tc.nonce)
		}
		if ec := ssr.GetErrorStatus().GetCode(); ec != int32(tc.errCode) {
			t.Errorf("%s : TargetStreamSet.Add(), err code was %v, want %v", tc.name, ec, tc.errCode)
		}
	}
}
