// Package proxy provides the client side API for working with a proxy server.
package proxy

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"

	proxypb "github.com/Snowflake-Labs/sansshell/proxy"
)

type ProxyConn struct {
	// The targets we're proxying for currently.
	targets []string
	// The RPC connection to the proxy.
	cc *grpc.ClientConn
	// The current unused nonce. Always incrementing as we send requests.
	nonce uint32

	// Protects nonce
	mu sync.Mutex
}

// Invoke implements grpc.ClientConnInterface
func (p *ProxyConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	if len(p.targets) != 1 {
		return status.Error(codes.InvalidArgument, "cannot invoke 1:1 RPC's with multiple targets")
	}

	requestMsg, ok := args.(proto.Message)
	if !ok {
		return status.Error(codes.InvalidArgument, "args must be a proto.Message")
	}
	replyMsg, ok := reply.(proto.Message)
	if !ok {
		return status.Error(codes.InvalidArgument, "reply must be a proto.Message")
	}

	nonce := p.getNonce()
	req := &proxypb.ProxyRequest{
		Request: &proxypb.ProxyRequest_StartStream{
			StartStream: &proxypb.StartStream{
				Target:     p.targets[0],
				MethodName: method,
				Nonce:      nonce,
			},
		},
	}

	stream, err := proxypb.NewProxyClient(p.cc).Proxy(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "can't setup proxy stream - %v", err)
	}
	err = stream.Send(req)
	if err != nil {
		return status.Errorf(codes.Internal, "can't send request for %s on stream - %v", method, err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "can't get response for %s on stream - %v", method, err)
	}

	// Validate we got an answer and it has expected reflected values.
	r := resp.GetStartStreamReply()
	if r == nil {
		return status.Errorf(codes.Internal, "didn't get expected start stream reply for %s on stream - %v", method, err)
	}

	if s := r.GetErrorStatus(); s != nil {
		return status.Errorf(codes.Internal, "got error from stream. Code: %s Message: %s", codes.Code(s.Code).String(), s.Message)
	}

	if gotTarget, wantTarget, gotNonce, wantNonce := r.Target, req.GetStartStream().Target, r.Nonce, req.GetStartStream().Nonce; gotTarget != wantTarget || gotNonce != wantNonce {
		return status.Errorf(codes.Internal, "didn't get matching target/nonce from stream reply. got %s/%d want %s/%d", gotTarget, gotNonce, wantTarget, wantNonce)
	}

	// Save stream ID for later matching.
	streamId := r.GetStreamId()

	anydata, err := anypb.New(requestMsg)
	if err != nil {
		return status.Errorf(codes.Internal, "can't marshall request data to Any - %v", err)
	}

	// Now send the request down.
	data := &proxypb.ProxyRequest{
		Request: &proxypb.ProxyRequest_StreamData{
			StreamData: &proxypb.StreamData{
				StreamIds: []uint64{streamId},
				Payload:   anydata,
			},
		},
	}
	err = stream.Send(data)
	if err != nil {
		return status.Errorf(codes.Internal, "can't send request data for %s on stream - %v", method, err)
	}

	// Get the response back which should be the unary RPC response.
	resp, err = stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "can't get response data for %s on stream - %v", method, err)
	}

	switch {
	case resp.GetStreamData() != nil:
		// Validate it's the stream we want.
		ids := resp.GetStreamData().StreamIds
		if len(ids) != 1 || ids[0] != streamId {
			return status.Errorf(codes.Internal, "didn't get back expected stream id %d. Got %+v instead", streamId, ids)
		}
		// What we expected - an answer to the RPC.
		err = resp.GetStreamData().Payload.UnmarshalTo(replyMsg)
		if err != nil {
			return status.Errorf(codes.Internal, "can't unmarshall Any -> reply - %v", err)
		}
	case resp.GetServerClose() != nil:
		code := resp.GetServerClose().Status.Code
		msg := resp.GetServerClose().Status.Message
		return status.Errorf(codes.Internal, "didn't get reply back. Server closed with code: %s message: %s", codes.Code(code).String(), msg)
	}

	// Ok, one more receive for the server close.
	resp, err = stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "can't get response data for %s on stream - %v", method, err)
	}
	close := resp.GetServerClose()
	if close == nil {
		return status.Error(codes.Internal, "didn't get ServerClose message as expected on stream")
	}
	ids := close.StreamIds
	if len(ids) != 1 || ids[0] != streamId {
		return status.Errorf(codes.Internal, "ServerClose for wrong stream. Want %d and got %v", streamId, ids)
	}

	// No error, we're good.
	if close.GetStatus().GetCode() == int32(codes.OK) {
		return nil
	}

	return status.Errorf(codes.Internal, "Stream ended with error. Code: %s message: %s", codes.Code(close.Status.Code).String(), close.Status.Message)
}

func (p *ProxyConn) getNonce() uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nonce++
	return p.nonce
}

// NewStream implements grpc.ClientConnInterface
func (p *ProxyConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// Additional methods needed for 1:N and N:N below.
type ProxyRet struct {
	Target string
	Ret    anypb.Any
	Error  error
}

// InvokeOneMany is used in proto generated code to implemened unary OneMany methods doing 1:N calls to the proxy.
func (p *ProxyConn) InvokeOneMany(ctx context.Context, method string, args interface{}, opts ...grpc.CallOption) (<-chan *ProxyRet, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// Close tears down the ProxyConn and closes all connections to it.
func (p *ProxyConn) Close() error {
	return p.cc.Close()
}

// Dial will connect to the given proxy and setup to send RPCs to the listed targets.
// If proxy is blank and there is only one target this will return a normal grpc connection object (*grpc.ClientConn).
// Otherwise this will return a *ProxyConn setup to act with the proxy.
func Dial(proxy string, targets []string, opts ...grpc.DialOption) (grpc.ClientConnInterface, error) {
	return DialContext(context.Background(), proxy, targets, opts...)
}

// DialContext is the same as Dial except the context provided can be used to cancel or expire the pending connection.
// By default dial operations are non-blocking. See grpc.Dial for a complete explanation.
func DialContext(ctx context.Context, proxy string, targets []string, opts ...grpc.DialOption) (grpc.ClientConnInterface, error) {
	if proxy == "" {
		if len(targets) != 1 {
			return nil, status.Error(codes.InvalidArgument, "no proxy specified but more than one target set")
		}
		return grpc.DialContext(ctx, targets[0], opts...)
	}
	if len(targets) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no targets passed")
	}
	conn, err := grpc.DialContext(ctx, proxy, opts...)
	if err != nil {
		return nil, err
	}
	ret := &ProxyConn{
		cc: conn,
	}
	// Make our own copy of these.
	ret.targets = append(ret.targets, targets...)
	return ret, nil
}
