// package testutil contains helpers and utilities for
// writing unittests against the sansshell proxy.
package testutil

import (
	"fmt"
	"math/rand"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	pb "github.com/Snowflake-Labs/sansshell/proxy"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

// Exchange is a test helper for the common pattern of trading messages with
// a proxy server over an open stream.
// Errors encountered during send/receive will cause `t` to fail.
// `req` may be nil, in which case this only performs a Recv
func Exchange(t *testing.T, stream pb.Proxy_ProxyClient, req *pb.ProxyRequest) *pb.ProxyReply {
	t.Helper()
	if req != nil {
		err := stream.Send(req)
		testutil.FatalOnErr(fmt.Sprintf("ProxyClient.Send(%v)", req), err, t)
	}
	reply, err := stream.Recv()
	testutil.FatalOnErr("ProxyClient.Recv()", err, t)
	return reply
}

// StartTargetStream establishes a new target stream through the proxy connection in `stream`.
// Will fail `t` on any errors communicating with the proxy, or if the returned response from
// the proxy is not a valid StartStreamReply.
func StartStream(t *testing.T, stream pb.Proxy_ProxyClient, target, method string) *pb.StartStreamReply {
	t.Helper()
	nonce := rand.Uint32()
	req := &pb.ProxyRequest{
		Request: &pb.ProxyRequest_StartStream{
			StartStream: &pb.StartStream{
				Target:     target,
				MethodName: method,
				Nonce:      nonce,
			},
		},
	}
	reply := Exchange(t, stream, req)
	switch reply.Reply.(type) {
	case *pb.ProxyReply_StartStreamReply:
		ssr := reply.GetStartStreamReply()
		if ssr.Nonce != nonce {
			t.Fatalf("StartStream(%s, %s) mismatched nonce, want %d, got %d", target, method, nonce, ssr.Nonce)
		}
		return ssr
	default:
		t.Fatalf("StartStream(%s, %s) got reply of type %T, want StartStreamReply", target, method, reply.Reply)
	}
	return nil
}

// MustStartStream invokes StartStream, but fails `t` if the response does not contain a valid
// stream id. Returns the created stream id.
func MustStartStream(t *testing.T, stream pb.Proxy_ProxyClient, target, method string) uint64 {
	t.Helper()
	reply := StartStream(t, stream, target, method)
	if reply.GetStreamId() == 0 {
		t.Fatalf("MustStartStream(%s, %s) want response with valid stream ID, got %+v", target, method, reply)
	}
	return reply.GetStreamId()
}

// PackStreamData creates a StreamData request for the supplied streamIds, with `req` as
// the payload.
// Any error in creation will fail `t`
func PackStreamData(t *testing.T, req proto.Message, streamIds ...uint64) *pb.ProxyRequest {
	t.Helper()
	packed, err := anypb.New(req)
	testutil.FatalOnErr(fmt.Sprintf("anypb.New(%+v)", req), err, t)
	return &pb.ProxyRequest{
		Request: &pb.ProxyRequest_StreamData{
			StreamData: &pb.StreamData{
				StreamIds: streamIds,
				Payload:   packed,
			},
		},
	}
}

func UnpackStreamData(t *testing.T, reply *pb.ProxyReply) ([]uint64, proto.Message) {
	t.Helper()
	sd := reply.GetStreamData()
	if sd == nil {
		t.Fatalf("UnpackStreamData() reply was of type %T, want StreamData", reply.Reply)
	}
	data, err := sd.Payload.UnmarshalNew()
	testutil.FatalOnErr(fmt.Sprintf("anypb.UnmarshalNew(%v)", sd), err, t)
	return sd.StreamIds, data
}
