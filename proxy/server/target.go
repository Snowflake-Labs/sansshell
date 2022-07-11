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
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	pb "github.com/Snowflake-Labs/sansshell/proxy"
)

var (
	// ReqBufferSize is the amount of requests we'll buffer on a given stream
	// while blocking to do the initial connect. After this the whole stream
	// will block until it can proceed (or error). Exported as a var so it can
	// be bound to a flag if wanted. By default this is only one as most RPCs
	// are unary from the client end so a small buffer is fine. Larger numbers
	// can cause large explosions in memory usage as potentially needing to buffer
	// N requests per sub stream that is slow/timing out.
	ReqBufferSize = 1
)

// A TargetStream is a single bidirectional stream between
// the proxy and a target sansshell server
type TargetStream struct {
	// The parent context that owns this stream.
	ctx context.Context

	// The unique indentifier of this stream
	streamID uint64

	// The address of the target server, as given in StartStream
	target string

	// The ServiceMethod spec
	serviceMethod *ServiceMethod

	// The underlying grpc.ClientConnInterface to the target server
	grpcConn grpc.ClientConnInterface

	// The underlying grpc.ClientStream to the target server.
	grpcStream grpc.ClientStream

	// streamMu guards access to grpcStream.
	streamMu sync.RWMutex

	// A cancel function that can be used to request early cancellation
	// of the stream context
	cancelFunc context.CancelFunc

	// A 'once' guarding CloseSend to allow it to be called idempotently
	closeOnce sync.Once

	// The (internal) channel used to manage incoming requests
	reqChan chan proto.Message

	// A channel used to carry an error from proxy-initiated closure
	errChan chan error

	// A logger used to log additional information
	logger logr.Logger

	// The authorizer (from the stream set) used to OPA check requests
	// sent to this stream.
	authorizer *rpcauth.Authorizer

	// The dialer to use for connecting to targets.
	dialer TargetDialer

	// If this is set it will be used as a blocking dial timeout to
	// the remote target.
	dialTimeout *time.Duration
}

func (s *TargetStream) getStream() grpc.ClientStream {
	s.streamMu.RLock()
	defer s.streamMu.RUnlock()
	return s.grpcStream
}

func (s *TargetStream) setStream(stream grpc.ClientStream) {
	s.streamMu.Lock()
	s.grpcStream = stream
	s.streamMu.Unlock()
}

func (s *TargetStream) String() string {
	return fmt.Sprintf("Stream(id:%d target:%s, method:%s)", s.streamID, s.target, s.serviceMethod.FullName())
}

// StreamID returns the proxy-assigned stream identifier for this
// stream
func (s *TargetStream) StreamID() uint64 {
	return s.streamID
}

// Method returns the full method name associated with this stream
func (s *TargetStream) Method() string {
	return s.serviceMethod.FullName()
}

// Target returns the address of the target
func (s *TargetStream) Target() string {
	return s.target
}

// PeerAuthInfo returns authz-relevant information about the stream peer
func (s *TargetStream) PeerAuthInfo() *rpcauth.PeerAuthInput {
	return rpcauth.PeerInputFromContext(s.getStream().Context())
}

// NewRequest returns a new, empty request message for this target stream.
func (s *TargetStream) NewRequest() proto.Message {
	return s.serviceMethod.NewRequest()
}

// CloseSend is used to indicate that no more client requests
// will be sent to this stream
func (s *TargetStream) CloseSend() {
	s.closeOnce.Do(func() {
		close(s.reqChan)
	})
}

// ClientCancel requests cancellation of the stream
func (s *TargetStream) ClientCancel() {
	s.cancelFunc()
}

// CloseWith initiates a closer of the stream, with the supplied
// error delivered in the ServerClose message, if no status has
// already been sent.
// If `err` is convertible to a grpc.Status, the status code
// will be preserved.
func (s *TargetStream) CloseWith(err error) {
	select {
	case s.errChan <- err:
		s.cancelFunc()
	default:
		// an error is already pending. Do nothing
	}
}

// Send the supplied request to the target stream, returning
// an error if the context has already been cancelled.
func (s *TargetStream) Send(req proto.Message) error {
	ctx := s.getStream().Context()
	select {
	case s.reqChan <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run begins execution of the target stream
// All data received from target will be converted into ProxyReply
// messages for sending to a proxy client, including the final
// status of the target stream
func (s *TargetStream) Run(nonce uint32, replyChan chan *pb.ProxyReply) {
	group, ctx := errgroup.WithContext(s.ctx)

	group.Go(func() error {
		dialCtx, cancel := context.WithCancel(ctx)
		var opts []grpc.DialOption
		if s.dialTimeout != nil {
			dialCtx, cancel = context.WithTimeout(ctx, *s.dialTimeout)
			opts = append(opts, grpc.WithBlock())
		}
		var err error
		defer cancel()
		s.grpcConn, err = s.dialer.DialContext(dialCtx, s.target, opts...)
		if err != nil {
			// We cannot create a new stream to the target. So we need to cancel this stream.
			s.logger.Info("unable to create stream", "status", err)
			s.cancelFunc()
			return err
		}
		grpcStream, err := s.grpcConn.NewStream(s.ctx, s.serviceMethod.StreamDesc(), s.serviceMethod.FullName())
		if err != nil {
			// We cannot create a new stream to the target. So we need to cancel this stream.
			s.logger.Info("unable to create stream", "status", err)
			return err
		}

		// We've successfully connected and can replace the initial unconnected stream
		// with the target stream.
		s.setStream(grpcStream)

		// Receives messages from the server stream
		group.Go(func() error {
			for {
				msg := s.serviceMethod.NewReply()
				err := grpcStream.RecvMsg(msg)
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
				}
				// otherwise, this is a streamData reply
				packed, err := anypb.New(msg)
				if err != nil {
					return err
				}
				reply := &pb.ProxyReply{
					Reply: &pb.ProxyReply_StreamData{
						StreamData: &pb.StreamData{
							StreamIds: []uint64{s.streamID},
							Payload:   packed,
						},
					},
				}
				replyChan <- reply
			}
		})
		// read from the incoming request channel, and write
		// to the stream.
		for {
			var req proto.Message
			var ok bool
			select {
			case <-ctx.Done():
				return ctx.Err()
			case req, ok = <-s.reqChan:
				if !ok {
					// reqChan was closed, meaning no more messages are incoming
					// if this was a client stream, we issue a half-close
					if s.serviceMethod.ClientStreams() {
						err = grpcStream.CloseSend()
						if err != nil {
							s.CloseWith(err)
						}
						return err
					}
					// otherwise, we're done
					return nil
				}
			}

			authinput, err := rpcauth.NewRPCAuthInput(ctx, s.Method(), req)
			if err != nil {
				err = status.Errorf(codes.Internal, "error creating authz input %v", err)
				s.CloseWith(err)
				return err
			}
			streamPeerInfo := s.PeerAuthInfo()
			authinput.Host = &rpcauth.HostAuthInput{
				Net: streamPeerInfo.Net,
			}

			// If authz fails, close immediately with an error
			if err := s.authorizer.Eval(ctx, authinput); err != nil {
				s.CloseWith(err)
				return err
			}

			err = grpcStream.SendMsg(req)
			// if this returns an EOF, then the final status
			// will be returned via a call to RecvMsg, and we
			// should not return an error here, since that
			// would cancel the errgroup early. Instead, we
			// can return nil, and the error will be picked
			// up by the receiving goroutine
			if err == io.EOF {
				return nil
			}
			// Otherwise, this is the 'final' error. The underlying
			// stream will be torn down automatically, but we
			// can return the error here, where it will be returned
			// by the errgroup, and sent in the ServerClose
			if err != nil {
				s.cancelFunc()
				return err
			}
			// no error. If client streaming is not expected, then we're
			// done
			if !s.serviceMethod.ClientStreams() {
				return nil
			}
		}
	})
	// Wait for final status from the errgroup, and translate it into
	// a server-close call
	err := group.Wait()

	// The error status may by set/overidden if CloseWith was used to
	// terminate the stream.
	select {
	case err = <-s.errChan:
	default:
	}
	s.logger.Info("finished", "status", err)
	reply := &pb.ProxyReply{
		Reply: &pb.ProxyReply_ServerClose{
			ServerClose: &pb.ServerClose{
				StreamIds: []uint64{s.streamID},
				Status:    convertStatus(status.Convert(err)),
			},
		},
	}
	replyChan <- reply
}

// NewTargetStream creates a new TargetStream for calling `method` on `target`
func NewTargetStream(ctx context.Context, target string, dialer TargetDialer, dialTimeout *time.Duration, method *ServiceMethod, authorizer *rpcauth.Authorizer) (*TargetStream, error) {
	logger := logr.FromContextOrDiscard(ctx)
	ctx, cancel := context.WithCancel(ctx)

	ts := &TargetStream{
		ctx:           ctx,
		authorizer:    authorizer,
		streamID:      rand.Uint64(),
		target:        target,
		serviceMethod: method,
		grpcStream:    &unconnectedClientStream{ctx: ctx},
		cancelFunc:    cancel,
		reqChan:       make(chan proto.Message, ReqBufferSize),
		errChan:       make(chan error, 1),
		dialer:        dialer,
		dialTimeout:   dialTimeout,
	}
	ts.logger = logger.WithValues("stream", ts.String())
	ts.logger.Info("created")
	return ts, nil
}

// A TargetStreamSet manages multiple TargetStreams
// TargetStreamSets are not threadsafe, and should not
// be shared by multiple goroutines without external
// synchronization
type TargetStreamSet struct {
	// A service method map used to resolve incoming stream requests to service methods
	serviceMethods map[string]*ServiceMethod

	// A TargetDialer for initiating target connections
	targetDialer TargetDialer

	// an Authorizer, for authorizing requests sent to targets.
	authorizer *rpcauth.Authorizer

	// The set of streams managed by this set
	streams map[uint64]*TargetStream

	// The streams we've previously had open but have since closed.
	closedStreams map[uint64]bool

	// A WaitGroup used to track active streams
	wg sync.WaitGroup

	// A set of "target|nonce" strings, used to track previously
	// seen target/nonce pairs to prevent inadvertent re-use.
	noncePairs map[string]bool
}

// NewTargetStreamSet creates a TargetStreamSet which manages a set of related TargetStreams
func NewTargetStreamSet(serviceMethods map[string]*ServiceMethod, dialer TargetDialer, authorizer *rpcauth.Authorizer) *TargetStreamSet {
	return &TargetStreamSet{
		serviceMethods: serviceMethods,
		targetDialer:   dialer,
		authorizer:     authorizer,
		streams:        make(map[uint64]*TargetStream),
		closedStreams:  make(map[uint64]bool),
		noncePairs:     make(map[string]bool),
	}
}

// Add creates a new target stream for the given start stream request, and adds it to the set of streams
// managed by this set
//
// The result of stream creation, as well as any messages received from the created stream will be
// sent directly to 'replyChan'. If the stream was successfully started, its id will eventually be
// sent to 'doneChan' when all work has completed.
//
// Returns a non-nil error only on unrecoverable client error, such as the re-use of a nonce/target
// pair, which cannot be represented by a stream-specific status.
func (t *TargetStreamSet) Add(ctx context.Context, req *pb.StartStream, replyChan chan *pb.ProxyReply, doneChan chan uint64) error {
	// Check for client reuse of a previously used target/nonce pair, to avoid
	// the case in which a buggy client sends multiple StartStream request with
	// the same target and nonce, and is uanble to disambiguate the responses.
	// NB: while the check for re-use is here, the target/nonce pair is not
	// considered to have been used unless we successfully return a stream id
	// to the client. This permits clients to retry failing requests wihout
	// needing to increment the nonce on each attempt.
	targetNonce := fmt.Sprintf("%s|%d", req.GetTarget(), req.GetNonce())
	if t.noncePairs[targetNonce] {
		// Unlike most of the errors in this function, a nonce-reuse error will cause this function
		// to return with an error, rather than sending a response on the reply channel, since
		// the client would have no way to distinguish the request that generated the error.
		return status.Errorf(codes.FailedPrecondition, "re-use of previous (target,nonce) (%s,%d)", req.GetTarget(), req.GetNonce())

	}
	sendReply := func(msg *pb.ProxyReply) {
		select {
		case replyChan <- msg:
			// nothing
		case <-ctx.Done():
		}
	}
	reply := &pb.ProxyReply{
		Reply: &pb.ProxyReply_StartStreamReply{
			StartStreamReply: &pb.StartStreamReply{
				Target: req.Target,
				Nonce:  req.Nonce,
			},
		},
	}
	serviceMethod, ok := t.serviceMethods[req.GetMethodName()]
	if !ok {
		reply.GetStartStreamReply().Reply = &pb.StartStreamReply_ErrorStatus{
			ErrorStatus: convertStatus(status.Newf(codes.InvalidArgument, "unknown method %s", req.GetMethodName())),
		}
		sendReply(reply)
		return nil
	}
	var dialTimeout *time.Duration
	if req.DialTimeout != nil {
		d := req.DialTimeout.AsDuration()
		dialTimeout = &d
	}
	stream, err := NewTargetStream(ctx, req.GetTarget(), t.targetDialer, dialTimeout, serviceMethod, t.authorizer)
	if err != nil {
		reply.GetStartStreamReply().Reply = &pb.StartStreamReply_ErrorStatus{
			ErrorStatus: convertStatus(status.New(codes.Internal, err.Error())),
		}
		sendReply(reply)
		return nil
	}
	streamID := stream.StreamID()
	t.streams[streamID] = stream
	t.noncePairs[targetNonce] = true
	t.wg.Add(1)
	reply.GetStartStreamReply().Reply = &pb.StartStreamReply_StreamId{
		StreamId: stream.StreamID(),
	}
	// Send back the reply as we have an ID. Everything happens below in Run.
	sendReply(reply)
	// Create a new go-routine to execute the stream, which
	// including making the initial connection to the target
	// (which is blocking, and cannot be done here without
	// blocking other incoming requests from the client).
	go func() {
		stream.Run(req.GetNonce(), replyChan)
		select {
		case doneChan <- streamID:
			// we notified caller of our status
		case <-ctx.Done():
			// or calling context is done
		}
		t.wg.Done()
	}()
	return nil
}

// Remove the stream corresponding to `streamid` from the
// stream set. Future references to this stream will return
// an error generally.
func (t *TargetStreamSet) Remove(streamID uint64) {
	delete(t.streams, streamID)
	t.closedStreams[streamID] = true
}

// Wait blocks until all TargetStreams associated with this
// stream set have completed.
func (t *TargetStreamSet) Wait() {
	t.wg.Wait()
}

// ClientClose dispatches ClientClose requests to TargetStreams
// identified by ID in `req`.
func (t *TargetStreamSet) ClientClose(req *pb.ClientClose) error {
	for _, id := range req.StreamIds {
		stream, ok := t.streams[id]
		if !ok {
			// NB:
			// Unlike other operations that accept a stream ID, ClientClosing
			// a non-existent stream isn't a fatal error. This mirrors the native
			// behavior of gRPC, which permits multiple calls to CloseSend, or
			// a CloseSend on a stream which has already sent its last request.
			continue
		}
		stream.CloseSend()
	}
	return nil
}

// ClientCloseAll issues ClientClose to all associated TargetStreams
func (t *TargetStreamSet) ClientCloseAll() {
	for _, stream := range t.streams {
		stream.CloseSend()
	}
}

// ClientCancel cancels TargetStreams identified by ID in `req`
func (t *TargetStreamSet) ClientCancel(req *pb.ClientCancel) error {
	for _, id := range req.StreamIds {
		stream, ok := t.streams[id]
		// Ordering problems means a client may dispatch data while a Close is
		// in flight. If we've previously had this stream we'll just ignore it.
		if !ok {
			if t.closedStreams[id] {
				continue
			}
			return status.Errorf(codes.InvalidArgument, "no such stream: %d", id)
		}
		stream.ClientCancel()
	}
	return nil
}

// Send dispatches new message data in `req` to the streams specified in req.
// It will return an error if a requested stream does not exist, the message
// type for the stream is incorrect, or if authorization data for the request
// cannot be generated.
// Before dispatching to the stream(s), an authorization check will be made to
// ensure that the request is permitted for all specified streams. On failure,
// streams that failed authorization will be closed with PermissionDenied,
// while other streams in the same request will continue along. Any other open
// TargetStreams which are not specified in the request are unaffected.
func (t *TargetStreamSet) Send(ctx context.Context, req *pb.StreamData) error {
	streamReq, err := req.Payload.UnmarshalNew()
	if err != nil {
		return status.Errorf(codes.Internal, "error unmarshalling request %v", err)
	}
	msgName := proto.MessageName(streamReq)

	var ids []uint64
	for _, id := range req.StreamIds {
		stream, ok := t.streams[id]
		// Ordering problems means a client may dispatch data while a Close is
		// in flight. If we've previously had this stream we'll just ignore it.
		if !ok {
			if t.closedStreams[id] {
				continue
			}
			return status.Errorf(codes.InvalidArgument, "no such stream: %d", id)
		}

		if msgName != proto.MessageName(stream.NewRequest()) {
			return status.Errorf(codes.InvalidArgument, "invalid request type for method %s", stream.Method())
		}
		// At this point it has passed checks so we can Send below.
		// A separate list is maintained as we might be silently dropping ids above due to a closed stream.
		ids = append(ids, id)
	}

	// All checks succeeded, send to all streams
	for _, id := range ids {
		reqClone := proto.Clone(streamReq)
		// TargetStream send only enqueues the message to the stream, and only fails
		// if the stream is being torn down, and is unable to accept it.
		if err := t.streams[id].Send(reqClone); err != nil {
			return err
		}
	}

	return nil
}

func convertStatus(s *status.Status) *pb.Status {
	if s == nil {
		return nil
	}
	p := s.Proto()
	return &pb.Status{
		Code:    p.Code,
		Message: p.Message,
		Details: p.Details,
	}
}

// An unconnected client stream is a grpc.ClientStream
// which is associated with a TargetStream prior to the
// successful establishment of a connection with the target.
// This removes the need for TargetStream code to handle possibly
// nil stream values prior to connection in Run.
type unconnectedClientStream struct {
	ctx context.Context
}

var (
	errUnconnectedClient = errors.New("unconnected stream")
)

// see: grpc.ClientStream.Header()
func (u *unconnectedClientStream) Header() (metadata.MD, error) {
	return nil, errUnconnectedClient
}

// see: grpc.ClientStream.Trailer()
func (u *unconnectedClientStream) Trailer() metadata.MD {
	return nil
}

// see: grpc.ClientStream.CloseSend()
func (u *unconnectedClientStream) CloseSend() error {
	return fmt.Errorf("%w: CloseSend", errUnconnectedClient)
}

// see: grpc.ClientStream.Context()
func (u *unconnectedClientStream) Context() context.Context {
	return u.ctx
}

// see: grpc.ClientStream.SendMsg()
func (u *unconnectedClientStream) SendMsg(interface{}) error {
	return fmt.Errorf("%w: SendMsg", errUnconnectedClient)
}

// see: grpc.ClientStream.RecvMsg()
func (u *unconnectedClientStream) RecvMsg(interface{}) error {
	return fmt.Errorf("%w: RecvMsg", errUnconnectedClient)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
