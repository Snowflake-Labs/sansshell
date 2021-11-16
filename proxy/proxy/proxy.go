// Package proxy provides the client side API for working with a proxy server.
//
// If called without a proxy simply acts as a pass though and normal ClientConnInterface.
package proxy

import (
	"context"
	"io"
	"log"
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
	Targets []string

	// The RPC connection to the proxy.
	cc *grpc.ClientConn

	// The current unused nonce. Always incrementing as we send requests.
	nonce uint32

	// If this is true we're not proxy but instead direct connect.
	direct bool

	// Protects nonce
	mu sync.Mutex
}

func (p *ProxyConn) getNonce() uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nonce++
	return p.nonce
}

// Direct indicates whether the proxy is in use or a direct connection is being made.
func (p *ProxyConn) Direct() bool {
	return p.direct
}

// NumTargets returns the number of targets ProxyConn is addressing (through the proxy or not).
func (p *ProxyConn) NumTargets() int {
	return len(p.Targets)
}

// Invoke implements grpc.ClientConnInterface
func (p *ProxyConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	if p.Direct() {
		// TODO(jchacon): Add V1 style logging indicating pass through in use.
		return p.cc.Invoke(ctx, method, args, reply, opts...)
	}
	if p.NumTargets() != 1 {
		return status.Error(codes.InvalidArgument, "cannot invoke 1:1 RPC's with multiple targets")
	}

	// This is just the degenerate case of OneMany with a single target. Just use it to do all the heavy lifting.
	retChan, err := p.InvokeOneMany(ctx, method, args, opts...)
	if err != nil {
		return status.Errorf(codes.Internal, "Calling InvokeOneMany with 1 request error - %v", err)
	}

	// In case we return before this is drained make sure it gets drained (and therefore closed)
	defer func() {
		for m := range retChan {
			log.Printf("Discarding msg: %+v", m)
		}
	}()

	replyMsg, ok := reply.(proto.Message)
	if !ok {
		return status.Error(codes.InvalidArgument, "reply must be a proto.Message")
	}

	// We should only get one response.
	gotResponse := false
	for resp := range retChan {
		if gotResponse {
			return status.Errorf(codes.Internal, "Got a 2nd response from InvokeMany for 1 request - %+v", resp)
		}

		gotResponse = true
		if resp.Error != nil {
			return resp.Error
		}
		if got, want := resp.Target, p.Targets[0]; got != want {
			return status.Errorf(codes.Internal, "Response for wrong target. Want %s and got %s", want, got)
		}
		if err := resp.Resp.UnmarshalTo(replyMsg); err != nil {
			return status.Errorf(codes.Internal, "Can't decode response - %v", err)
		}
	}

	if !gotResponse {
		return status.Errorf(codes.Internal, "Didn't get response for InvokeMany with 1 request")
	}
	return nil
}

// NewStream implements grpc.ClientConnInterface
func (p *ProxyConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	if p.direct {
		// TODO(jchacon): Add V1 style logging indicating pass through in use.
		return p.cc.NewStream(ctx, desc, method, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "NewStream not implemented for proxy")
}

// Additional methods needed for 1:N below.
type ProxyRet struct {
	Target string
	Resp   *anypb.Any
	Error  error
}

// InvokeOneMany is used in proto generated code to implemened unary OneMany methods doing 1:N calls to the proxy.
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (p *ProxyConn) InvokeOneMany(ctx context.Context, method string, args interface{}, opts ...grpc.CallOption) (<-chan *ProxyRet, error) {
	requestMsg, ok := args.(proto.Message)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "args must be a proto.Message")
	}

	stream, err := proxypb.NewProxyClient(p.cc).Proxy(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't setup proxy stream - %v", err)
	}

	streamIds := make(map[uint64]*ProxyRet)

	for _, t := range p.Targets {
		req := &proxypb.ProxyRequest{
			Request: &proxypb.ProxyRequest_StartStream{
				StartStream: &proxypb.StartStream{
					Target:     t,
					MethodName: method,
					Nonce:      p.getNonce(),
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
		streamIds[r.GetStreamId()] = &ProxyRet{
			Target: r.GetTarget(),
		}
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
			if err == io.EOF {
				break
			}
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
						chanErr = status.Errorf(codes.Internal, "unexpected stream id %d received for StreamData", id)
						break processing
					}
					streamIds[id].Resp = d.Payload
					retChan <- streamIds[id]
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
						if streamIds[id].Resp != nil {
							chanErr = status.Errorf(codes.Internal, "Got a non OK error code for a target we've already responded. Code: %s for target: %s", codes.Code(code).String(), streamIds[id].Target)
							// Remove it from the map so we don't double send responses for this target.
							delete(streamIds, id)
							break processing
						}

						streamIds[id].Error = status.Errorf(codes.Internal, "Server closed with code: %s message: %s", codes.Code(code).String(), msg)
						retChan <- streamIds[id]
					}
				}
				for _, id := range cl.StreamIds {
					delete(streamIds, id)
				}
			default:
				chanErr = status.Errorf(codes.Internal, "unexpected answer on stream. Wanted StreamData or ServerClose and got %+v instead", resp.Reply)
				break processing
			}

			// We've gotten closes for everything so we're done.
			if len(streamIds) == 0 {
				break
			}
		}

		// Any stream IDs left in the map haven't had a proper close on them so push the
		// current chanErr down to them.
		for _, msg := range streamIds {
			msg.Error = chanErr
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
func Dial(proxy string, targets []string, opts ...grpc.DialOption) (*ProxyConn, error) {
	return DialContext(context.Background(), proxy, targets, opts...)
}

// DialContext is the same as Dial except the context provided can be used to cancel or expire the pending connection.
// By default dial operations are non-blocking. See grpc.Dial for a complete explanation.
func DialContext(ctx context.Context, proxy string, targets []string, opts ...grpc.DialOption) (*ProxyConn, error) {
	if len(targets) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no targets passed")
	}

	dialTarget := proxy
	ret := &ProxyConn{}
	if proxy == "" {
		if len(targets) != 1 {
			return nil, status.Error(codes.InvalidArgument, "no proxy specified but more than one target set")
		}
		dialTarget = targets[0]
		ret.direct = true
	}
	conn, err := grpc.DialContext(ctx, dialTarget, opts...)
	if err != nil {
		return nil, err
	}
	ret.cc = conn
	// Make our own copy of these.
	ret.Targets = append(ret.Targets, targets...)
	return ret, nil
}
