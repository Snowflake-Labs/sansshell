// Package proxy provides the client side API for working with a proxy server.
package proxy

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

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
		code := resp.GetServerClose().GetStatus().GetCode()
		msg := resp.GetServerClose().GetStatus().GetMessage()
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
	return nil, status.Error(codes.Unimplemented, "NewStream not implemented")
}

// Additional methods needed for 1:N and N:N below.
type ProxyRet struct {
	Target string
	Resp   *anypb.Any
	Error  error
}

// InvokeOneMany is used in proto generated code to implemened unary OneMany methods doing 1:N calls to the proxy.
func (p *ProxyConn) InvokeOneMany(ctx context.Context, method string, args interface{}, opts ...grpc.CallOption) (<-chan *ProxyRet, error) {
	requestMsg, ok := args.(proto.Message)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "args must be a proto.Message")
	}

	stream, err := proxypb.NewProxyClient(p.cc).Proxy(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't setup proxy stream - %v", err)
	}

	nonces := make(map[string]uint32)
	streamIds := make(map[uint64]string)

	for _, t := range p.targets {
		nonces[t] = p.getNonce()
		req := &proxypb.ProxyRequest{
			Request: &proxypb.ProxyRequest_StartStream{
				StartStream: &proxypb.StartStream{
					Target:     t,
					MethodName: method,
					Nonce:      nonces[t],
				},
			},
		}

		err = stream.Send(req)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "can't send request for %s on stream - %v", method, err)
		}
		resp, err := stream.Recv()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "can't get response for %s on stream - %v", method, err)
		}

		// Validate we got an answer and it has expected reflected values.
		r := resp.GetStartStreamReply()
		if r == nil {
			return nil, status.Errorf(codes.Internal, "didn't get expected start stream reply for %s on stream - %v", method, err)
		}

		if s := r.GetErrorStatus(); s != nil {
			return nil, status.Errorf(codes.Internal, "got error from stream. Code: %s Message: %s", codes.Code(s.Code).String(), s.Message)
		}

		if gotTarget, wantTarget, gotNonce, wantNonce := r.Target, req.GetStartStream().Target, r.Nonce, req.GetStartStream().Nonce; gotTarget != wantTarget || gotNonce != wantNonce {
			return nil, status.Errorf(codes.Internal, "didn't get matching target/nonce from stream reply. got %s/%d want %s/%d", gotTarget, gotNonce, wantTarget, wantNonce)
		}

		// Save stream ID for later matching.
		streamIds[r.GetStreamId()] = r.GetTarget()
	}

	// Now construct a single request with all of our streamIds.
	anydata, err := anypb.New(requestMsg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't marshall request data to Any - %v", err)
	}

	var ids []uint64
	for s := range streamIds {
		ids = append(ids, s)
	}
	data := &proxypb.ProxyRequest{
		Request: &proxypb.ProxyRequest_StreamData{
			StreamData: &proxypb.StreamData{
				StreamIds: ids,
				Payload:   anydata,
			},
		},
	}

	// Now send the request down.
	err = stream.Send(data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't send request data for %s on stream - %v", method, err)
	}

	retChan := make(chan *ProxyRet)

	// Fire off a separate routine to read from the stream and send the responses down retChan.
	go func() {
		// An error that may occur in processing we'll do in bulk.
		var chanErr error

		// Now do receives until we get all the responses or closes for each stream ID.
	processing:
		for {
			resp, err := stream.Recv()
			if err != nil {
				chanErr = status.Errorf(codes.Internal, "can't get response data for %s on stream - %v", method, err)
				break
			}

			d := resp.GetStreamData()
			cl := resp.GetServerClose()

			switch {
			case d != nil:
				for _, id := range d.StreamIds {
					// Validate it's a stream we know.
					if _, ok := streamIds[id]; !ok {
						chanErr = status.Errorf(codes.Internal, "unexpected stream id %d received", id)
						break processing
					}
					msg := &ProxyRet{
						Target: streamIds[id],
						Resp:   d.Payload,
					}
					retChan <- msg
				}
			case cl != nil:
				code := cl.GetStatus().GetCode()
				msg := cl.GetStatus().GetMessage()

				// Do a one time check all the returned ids are ones we know.
				for _, id := range cl.StreamIds {
					if _, ok := streamIds[id]; !ok {
						chanErr = status.Errorf(codes.Internal, "unexpected stream id %d received", id)
						break processing
					}
				}

				// See if it's normal close. We can ignore those except to remove tracking.
				// Otherwise send errors back for each target and then remove.
				if code != int32(codes.OK) {
					for _, id := range cl.StreamIds {
						msg := &ProxyRet{
							Target: streamIds[id],
							Error:  status.Errorf(codes.Internal, "Server closed with code: %s message: %s", codes.Code(code).String(), msg),
						}
						retChan <- msg
					}
				}
				for _, id := range cl.StreamIds {
					delete(streamIds, id)
				}
			}

			// We've gotten closes for everything so we're done.
			if len(streamIds) == 0 {
				break
			}
		}

		// Any stream IDs left in the map haven't had a proper close on them so push the
		// current chanErr down to them.
		for _, t := range streamIds {
			msg := &ProxyRet{
				Target: t,
				Error:  chanErr,
			}
			retChan <- msg
		}
		close(retChan)
	}()

	// Any further error handling happens in-band with the responses.
	return retChan, nil
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
