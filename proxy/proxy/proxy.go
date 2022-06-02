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

// Package proxy provides the client side API for working with a proxy server.
//
// If called without a proxy simply acts as a pass though and normal ClientConnInterface.
package proxy

import (
	"context"
	"io"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	proxypb "github.com/Snowflake-Labs/sansshell/proxy"
)

// Conn is a grpc.ClientConnInterface which is connected to the proxy
// converting calls into RPC the proxy understands.
type Conn struct {
	// The targets we're proxying for currently.
	Targets []string

	// Possible dial timeouts for each target
	timeouts []*time.Duration

	// The RPC connection to the proxy.
	cc *grpc.ClientConn

	// If this is true we're not proxy but instead direct connect.
	direct bool
}

// Ret defines the internal API for getting responses from the proxy.
// Callers will need to convert the anypb.Any into their final type (generally via generated code).
type Ret struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *anypb.Any
	Error error
}

// Direct indicates whether the proxy is in use or a direct connection is being made.
func (p *Conn) Direct() bool {
	return p.direct
}

// Proxy will return the ClientConn which connects directly to the proxy rather
// than wrapped as proxy.Conn normally does. This allows callers to invoke direct RPCs
// against the proxy as needed (such as services/logging).
func (p *Conn) Proxy() *grpc.ClientConn {
	return p.cc
}

// proxyStream provides all the context for send/receive in a grpc stream sense then translated to the streaming connection
// we hold to the proxy. It also implements a fully functional grpc.ClientStream interface.
type proxyStream struct {
	method     string
	stream     proxypb.Proxy_ProxyClient
	ids        map[uint64]*Ret
	errors     []*Ret
	sentErrors bool
	sendClosed bool
}

// Invoke - see grpc.ClientConnInterface
func (p *Conn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	if p.Direct() {
		// TODO(jchacon): Add V1 style logging indicating pass through in use.
		return p.cc.Invoke(ctx, method, args, reply, opts...)
	}
	if len(p.Targets) != 1 {
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

// NewStream - see grpc.ClientConnInterface
func (p *Conn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	if p.direct {
		// TODO(jchacon): Add V1 style logging indicating pass through in use.
		return p.cc.NewStream(ctx, desc, method, opts...)
	}

	stream, streamIds, errors, err := p.createStreams(ctx, method)
	if err != nil {
		return nil, err
	}

	s := &proxyStream{
		method: method,
		stream: stream,
		ids:    streamIds,
		errors: errors,
	}

	return s, nil
}

// Header - see grpc.ClientStream
func (p *proxyStream) Header() (metadata.MD, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented for proxy")
}

// Trailer - see grpc.ClientStream
func (p *proxyStream) Trailer() metadata.MD {
	return nil
}

// CloseSend - see grpc.ClientStream
func (p *proxyStream) CloseSend() error {
	// Already closed, just return.
	if p.sendClosed {
		return nil
	}

	// Even if we get an error below there's nothing much to do so
	// mark it closed always.
	p.sendClosed = true

	// Close our end of it since we have nothing else to send.
	// The proxy will internally close send on each client stream as a result.
	if err := p.stream.CloseSend(); err != nil {
		return status.Errorf(codes.Internal, "can't send client close on stream - %v", err)
	}
	return nil
}

// Context - see grpc.ClientStream
func (p *proxyStream) Context() context.Context {
	return p.stream.Context()
}

// send provides a helper for bundling up a single request to the proxy to N hosts.
func (p *proxyStream) send(requestMsg proto.Message) error {
	// Now construct a single request with all of our streamIds.
	anydata, err := anypb.New(requestMsg)
	if err != nil {
		return status.Errorf(codes.Internal, "can't marshall request data to Any - %v", err)
	}

	var ids []uint64
	for s := range p.ids {
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

	// Send the request to the proxy.
	if err := p.stream.Send(data); err != nil {
		return status.Errorf(codes.Internal, "can't send request data for %s on stream - %v", p.method, err)
	}
	return nil
}

func (p *proxyStream) closeClients() error {
	var ids []uint64
	for s := range p.ids {
		ids = append(ids, s)
	}
	data := &proxypb.ProxyRequest{
		Request: &proxypb.ProxyRequest_ClientClose{
			ClientClose: &proxypb.ClientClose{
				StreamIds: ids,
			},
		},
	}
	// Send the request to the proxy. Only error if it's not EOF (session was closed).
	if err := p.stream.Send(data); err != nil && err != io.EOF {
		return status.Errorf(codes.Internal, "can't send close data for %s on stream - %v", p.method, err)
	}
	return nil
}

// see grpc.ClientStream
func (p *proxyStream) SendMsg(args interface{}) error {
	if p.sendClosed {
		return status.Error(codes.FailedPrecondition, "sending on a closed connection")
	}

	m, ok := args.(proto.Message)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "args for SendMsg must be a proto.Message %T", args)
	}

	// Now send the request down.
	return p.send(m)
}

// see grpc.ClientStream
func (p *proxyStream) RecvMsg(m interface{}) error {
	// Up front check for nothing left since we closed all streams.
	if len(p.ids) == 0 {
		return io.EOF
	}

	// Since the API is an interface{} we can change what this normally
	// expects from a proto.Message to a *[]*ProxyRet instead.
	//
	// Anything else is an error.
	manyRet, ok := m.(*[]*Ret)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "args for proxy RecvMsg must be a *[]*ProxyRet) - got %T", m)
	}

	// If we have any pre-canned errors push them on now.
	// Only send once or else the user gets spammed with errors for every Recv called.
	if !p.sentErrors {
		*manyRet = append(*manyRet, p.errors...)
		p.sentErrors = true
	}

	resp, err := p.stream.Recv()
	// If it's io.EOF the upper level code will handle that.
	if err != nil {
		return err
	}

	d := resp.GetStreamData()
	cl := resp.GetServerClose()

	switch {
	case d != nil:
		for _, id := range d.StreamIds {
			// Validate it's a stream we know.
			if _, ok := p.ids[id]; !ok {
				return status.Errorf(codes.Internal, "unexpected stream id %d received for StreamData", id)
			}
			p.ids[id].Resp = d.Payload
			p.ids[id].Error = nil
			*manyRet = append(*manyRet, p.ids[id])
		}
	case cl != nil:
		code := codes.Code(cl.GetStatus().GetCode())
		msg := cl.GetStatus().GetMessage()

		// Do a one time check all the returned ids are ones we know.
		for _, id := range cl.StreamIds {
			if _, ok := p.ids[id]; !ok {
				return status.Errorf(codes.Internal, "unexpected stream id %d received for ServerClose", id)
			}
		}

		// See if it's normal close. We can ignore those except to remove tracking.
		// Otherwise send errors back for each target and then remove.

		// A normal close actually returns this as an error so map it so clients know the stream closed.
		closedErr := io.EOF
		streamStatus := status.New(code, msg)

		if streamStatus.Code() != codes.OK {
			closedErr = streamStatus.Err()
		}
		for _, id := range cl.StreamIds {
			p.ids[id].Error = closedErr
			p.ids[id].Resp = nil
			*manyRet = append(*manyRet, p.ids[id])
			delete(p.ids, id)
		}
	default:
		return status.Errorf(codes.Internal, "unexpected answer on stream. Wanted StreamData or ServerClose and got %+v instead", resp.Reply)
	}
	return nil
}

// createStreams is a helper which does the heavy lifting of creating N tracked streams to the proxy
// for later RPCs to flow across. It returns a proxy stream object (for clients), and a map of stream ids to prefilled ProxyRet
// objects. If any of the targets had an error connecting these will be collected and returned as a slice. This way later calls
// can complete to the online hosts and return precanned errors for the offline ones.
// All Ret structs will have Index/Target already filled in so clients can map them to their requests.
func (p *Conn) createStreams(ctx context.Context, method string) (proxypb.Proxy_ProxyClient, map[uint64]*Ret, []*Ret, error) {
	var errors []*Ret
	stream, err := proxypb.NewProxyClient(p.cc).Proxy(ctx)
	if err != nil {
		return nil, nil, errors, status.Errorf(codes.Internal, "can't setup proxy stream - %v", err)
	}

	streamIds := make(map[uint64]*Ret)

	// For every target we have to send a separate StartStream (with a nonce which in our case is the target index so clients can map too).
	// We then validate the nonce matches and record the stream ID so later processing can match responses to the right targets.
	// This needs to be 2 loops as we want the server to process N StartStreams in parallel and then we'll loop getting responses.
	for i, t := range p.Targets {
		req := &proxypb.ProxyRequest{
			Request: &proxypb.ProxyRequest_StartStream{
				StartStream: &proxypb.StartStream{
					Target:     t,
					MethodName: method,
					Nonce:      uint32(i),
				},
			},
		}
		if p.timeouts[i] != nil {
			req.GetStartStream().DialTimeout = durationpb.New(*p.timeouts[i])
		}
		err = stream.Send(req)

		// If Send reports an error and is EOF we have to use Recv to get the actual error according to documentation
		// for SendMsg. However it appears SendMsg will return actual errors "sometimes" when it's the first stream
		// a server has ever handled so account for that here.
		if err != nil && err != io.EOF {
			return nil, nil, errors, status.Errorf(codes.Internal, "can't send request for %s on stream - %v", method, err)
		}
		if err != nil {
			_, err := stream.Recv()
			return nil, nil, errors, status.Errorf(codes.Internal, "remote error from Send for %s - %v", method, err)
		}
	}

	// We sent len(p.Targets) requests so loop for that many replies. If the server doesn't we'll have to wait until
	// our context times out then. If the server attempts something invalid we'll catch and just abort (i.e, duplicate
	// responses and/or out of range). We may encounter closes() in here mixed in with replies but we'll never get
	// more than that (data can't start until we return).
	replies := 0
	for replies != len(p.Targets) {
		resp, err := stream.Recv()
		if err != nil {
			return nil, nil, errors, status.Errorf(codes.Internal, "can't get response for %s on stream - %v", method, err)
		}

		// Validate we got an answer and it has expected reflected values.

		// These are all sanity checks for the entire session so an overall error is appropriate since we're likely
		// dealing with a broken proxy of some sort.
		switch t := resp.Reply.(type) {
		case *proxypb.ProxyReply_StartStreamReply:
			replies++
			r := t.StartStreamReply
			// We want the returned Target+nonce to match what we sent and that it's one we know about.
			if r.Nonce >= uint32(len(p.Targets)) {
				return nil, nil, errors, status.Errorf(codes.Internal, "got back invalid nonce (out of range): %+v", r)
			}
			if p.Targets[r.Nonce] != r.Target {
				return nil, nil, errors, status.Errorf(codes.Internal, "Target/nonce don't match. target %s(%d) is not %s: %+v", p.Targets[r.Nonce], r.Nonce, r.Target, r)
			}

			id := r.GetStreamId()
			if streamIds[id] != nil {
				return nil, nil, errors, status.Errorf(codes.Internal, "Duplicate response for target %s. Already have %+v for response %+v", r.Target, streamIds[id], r)
			}

			ret := &Ret{
				Target: r.GetTarget(),
				Index:  int(r.GetNonce()),
			}
			// If the target reported an error stick it in errors.
			if s := r.GetErrorStatus(); s != nil {
				ret.Error = status.Errorf(codes.Internal, "got reply error from stream. Code: %s Message: %s", codes.Code(s.Code), s.Message)
				errors = append(errors, ret)
				continue
			}

			// Save stream ID/nonce for later matching.
			streamIds[r.GetStreamId()] = ret
		case *proxypb.ProxyReply_ServerClose:
			c := t.ServerClose
			// We've never sent any data so a close here has to be an error.
			st := c.GetStatus()
			if st == nil || st.Code == 0 {
				return nil, nil, errors, status.Errorf(codes.Internal, "close with no data sent and no error? %+v", resp)
			}
			for _, id := range c.StreamIds {
				if streamIds[id] == nil {
					return nil, nil, errors, status.Errorf(codes.Internal, "close on invalid stream id: %+v", resp)
				}
				streamIds[id].Error = status.Errorf(codes.Internal, "got close error from stream. Code: %s Message: %s", codes.Code(st.Code), st.Message)
				errors = append(errors, streamIds[id])
				// If it's closed make sure we don't process it later on.
				delete(streamIds, id)
			}
		default:
			return nil, nil, errors, status.Errorf(codes.Internal, "unexpected reply for %s on stream - %+v", method, resp)
		}
	}
	return stream, streamIds, errors, nil
}

// InvokeOneMany is used in proto generated code to implemened unary OneMany methods doing 1:N calls to the proxy.
// This returns ProxyRet objects from the channel which contain anypb.Any so the caller (generally generated code)
// will need to convert those to the proper expected specific types.
//
// As we're attempting to invoke N RPC's together we do have an implicit blocking timeout for the remote server to
// get connections (or not) to the N targets. This is done in parallel but does mean there's a bound where the RPC
// may take up to a Dial timeout (usually 20s) if any target is unavailable. Therefore it's suggested this never be
// invoked with a context timeout lower than the remote server Dial timeout.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
//
// TODO(jchacon): Should add the ability to specify remote dial timeout in the connection to the proxy.
func (p *Conn) InvokeOneMany(ctx context.Context, method string, args interface{}, opts ...grpc.CallOption) (<-chan *Ret, error) {
	requestMsg, ok := args.(proto.Message)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "args must be a proto.Message")
	}

	stream, streamIds, errors, err := p.createStreams(ctx, method)
	if err != nil {
		return nil, err
	}

	s := &proxyStream{
		method: method,
		stream: stream,
		ids:    streamIds,
	}
	if err := s.send(requestMsg); err != nil {
		return nil, err
	}

	if err := s.closeClients(); err != nil {
		return nil, err
	}
	if err := s.CloseSend(); err != nil {
		return nil, err
	}
	retChan := make(chan *Ret)

	// Fire off a separate routine to read from the stream and send the responses down retChan.
	go func() {
		// An error that may occur in processing we'll do in bulk.
		var chanErr error

		// If we have any pre-canned target errors just ship those down.
		for _, e := range errors {
			retChan <- e
		}

		// Now do receives until we get all the responses or closes for each stream ID.
	processing:
		for {
			resp, err := s.stream.Recv()
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
					if _, ok := s.ids[id]; !ok {
						chanErr = status.Errorf(codes.Internal, "unexpected stream id %d received for StreamData", id)
						break processing
					}
					s.ids[id].Resp = d.Payload
					retChan <- s.ids[id]
				}
			case cl != nil:
				code := codes.Code(cl.GetStatus().GetCode())
				msg := cl.GetStatus().GetMessage()

				// Do a one time check all the returned ids are ones we know.
				for _, id := range cl.StreamIds {
					if _, ok := s.ids[id]; !ok {
						chanErr = status.Errorf(codes.Internal, "unexpected stream id %d received for ServerClose", id)
						break processing
					}
				}

				// See if it's normal close. We can ignore those except to remove tracking.
				// Otherwise send errors back for each target and then remove.
				if code != codes.OK {
					for _, id := range cl.StreamIds {
						if s.ids[id].Resp != nil {
							chanErr = status.Errorf(codes.Internal, "Got a non OK error code for a target we've already responded. Code: %s for target: %s (%d) - %s", code.String(), s.ids[id].Target, s.ids[id].Index, msg)
							// Remove it from the map so we don't double send responses for this target.
							delete(s.ids, id)
							break processing
						}

						s.ids[id].Error = status.New(code, msg).Err()
						retChan <- s.ids[id]
					}
				}
				for _, id := range cl.StreamIds {
					delete(s.ids, id)
				}
			default:
				chanErr = status.Errorf(codes.Internal, "unexpected answer on stream. Wanted StreamData or ServerClose and got %+v instead", resp.Reply)
				break processing
			}

			// We've gotten closes for everything so we're done.
			if len(s.ids) == 0 {
				break
			}
		}

		// Any stream IDs left in the map haven't had a proper close on them so push the
		// current chanErr down to them.
		for _, msg := range s.ids {
			msg.Error = chanErr
			retChan <- msg
		}
		close(retChan)
	}()

	// Any further error handling happens in-band with the responses.
	return retChan, nil
}

// Close tears down the ProxyConn and closes all connections to it.
func (p *Conn) Close() error {
	return p.cc.Close()
}

func parseTargets(targets []string) ([]string, []*time.Duration, error) {
	var hostport []string
	var timeouts []*time.Duration
	for _, t := range targets {
		if len(t) == 0 {
			return nil, nil, status.Error(codes.InvalidArgument, "blank targets are not allowed")
		}
		// First pull off any possible duration
		p := strings.Split(t, ";")
		if len(p) > 2 {
			return nil, nil, status.Errorf(codes.InvalidArgument, "target must be of the form host[:port][;N<time.Duration>] and %s is invalid", t)
		}
		if p[0] == "" {
			return nil, nil, status.Error(codes.InvalidArgument, "blank targets are not allowed")
		}
		// That's the most we'll parse target here. The rest can happen in Invoke/NewStream later.
		hostport = append(hostport, p[0])
		if len(p) == 1 {
			// No timeout, so just append a placeholder empty value.
			timeouts = append(timeouts, nil)
			continue
		}
		d, err := time.ParseDuration(p[1])
		if err != nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "%s invalid duration - %v", p[1], err)
		}
		timeouts = append(timeouts, &d)
	}
	return hostport, timeouts, nil
}

// Dial will connect to the given proxy and setup to send RPCs to the listed targets.
// If proxy is blank and there is only one target this will return a normal grpc connection object (*grpc.ClientConn).
// Otherwise this will return a *ProxyConn setup to act with the proxy. Targets is a list of normal gRPC style
// endpoint addresses with an optional dial timeout appended with a semi-colon in time.Duration format.
// i.e. host[:port][;Ns] for instance to set the dial timeout to N seconds. The proxy value can also specify a dial timeout
// in the same fashion.
func Dial(proxy string, targets []string, opts ...grpc.DialOption) (*Conn, error) {
	return DialContext(context.Background(), proxy, targets, opts...)
}

// DialContext is the same as Dial except the context provided can be used to cancel or expire the pending connection.
// By default dial operations are non-blocking. See grpc.Dial for a complete explanation.
func DialContext(ctx context.Context, proxy string, targets []string, opts ...grpc.DialOption) (*Conn, error) {
	ret := &Conn{}
	parsedProxy := []string{proxy}
	proxyTimeout := []*time.Duration{nil}

	var err error
	if proxy != "" {
		parsedProxy, proxyTimeout, err = parseTargets([]string{proxy})
		if err != nil {
			return nil, err
		}
	}
	hostport, timeouts, err := parseTargets(targets)
	if err != nil {
		return nil, err
	}

	// If there are no targets things will likely fail but this gives the ability to still send RPCs to the
	// proxy itself.
	dialTarget := parsedProxy[0]
	dialTimeout := proxyTimeout[0]
	if proxy == "" {
		if len(targets) != 1 {
			return nil, status.Error(codes.InvalidArgument, "no proxy specified but more than one target set")
		}
		dialTarget = hostport[0]
		dialTimeout = timeouts[0]
		ret.direct = true

	}
	var cancel context.CancelFunc
	if dialTimeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *dialTimeout)
		opts = append(opts, grpc.WithBlock())
		defer cancel()
	}
	conn, err := grpc.DialContext(ctx, dialTarget, opts...)
	if err != nil {
		return nil, err
	}
	ret.cc = conn
	// Make our own copy of these.
	ret.Targets = append(ret.Targets, hostport...)
	ret.timeouts = append(ret.timeouts, timeouts...)
	return ret, nil
}
