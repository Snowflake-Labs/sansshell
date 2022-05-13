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

package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	pb "github.com/Snowflake-Labs/sansshell/proxy"
	_ "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

// A TargetDialer than returns an error for all Dials
type dialErrTargetDialer codes.Code

func (e dialErrTargetDialer) DialContext(ctx context.Context, target string) (grpc.ClientConnInterface, error) {
	return nil, status.Error(codes.Code(e), "")
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
			panic("TargetStreamSet.Wait() blocked on empty set, want no block")
		}
	}()
	ss.Wait()
	close(finishedWait)

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

	// Close of nonexistent ids is not an error, to match gRPC semantics which are tolerent of CloseSend on complete
	// connections.
	if err := ss.ClientClose(&pb.ClientClose{StreamIds: []uint64{1}}); status.Code(err) != codes.OK {
		t.Errorf("TargetStream.ClientClose(1) err code was %v, want code.OK", status.Code(err))
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.StartStream{
				Target:     "nosuchhost:000",
				Nonce:      tc.nonce,
				MethodName: tc.method,
			}
			err := ss.Add(context.Background(), req, replyChan, nil /*doneChan should not be called*/)
			testutil.FatalOnErr(fmt.Sprintf("StartStream(+%v)", req), err, t)
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
		})
	}
}

// a clientClonn that blocks until the calling context
// is cancelled.
type blockingClientConn struct{}

// see: grpc.ClientConnInterface.Invoke
func (b blockingClientConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	<-ctx.Done()
	return ctx.Err()
}

// see: grpc.ClientConnInterface.NewStream
func (b blockingClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// a context dialer that returns blockingClientConn
type blockingClientDialer struct{}

func (b blockingClientDialer) DialContext(ctx context.Context, target string) (grpc.ClientConnInterface, error) {
	return blockingClientConn{}, nil
}

func TestTargetStreamAddNonBlocking(t *testing.T) {
	// Adding a target to a stream set should not block.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceMap := LoadGlobalServiceMap()
	ss := NewTargetStreamSet(serviceMap, blockingClientDialer{}, nil)
	replyChan := make(chan *pb.ProxyReply, 1)
	doneChan := make(chan struct{})
	req := &pb.StartStream{
		Target:     "nosuchhost:000",
		MethodName: "/Testdata.TestService/TestUnary",
	}
	go func() {
		ss.Add(ctx, req, replyChan, nil)
		close(doneChan)
	}()
	select {
	case <-time.After(1 * time.Second):
		// we're blocked.
		t.Fatal("TargetStreamSet.Add blocked")
	case <-doneChan:
		// return
	}
}
