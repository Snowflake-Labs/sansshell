package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	pb "github.com/Snowflake-Labs/sansshell/proxy"
)

// A TargetStream is a single bidirectional stream between
// the proxy and a target sansshell server
type TargetStream struct {
	// The unique indentifier of this stream
	streamID uint64

	// The address of the target server, as given in StartStream
	target string

	// The ServiceMethod spec
	serviceMethod *ServiceMethod

	// The underlying grpc.ClientStream to the target server
	grpcStream grpc.ClientStream

	// A cancel function that can be used to request early cancellation
	// of the stream context
	cancelFunc context.CancelFunc

	// A 'once' guarding CloseSend to allow it to be called idempotently
	closeOnce sync.Once

	// the (internal) channel used to manage incoming requests
	reqChan chan proto.Message
}

func (s *TargetStream) String() string {
	return fmt.Sprintf("Stream(id:%d target:%s, method:%s)", s.streamID, s.target, s.serviceMethod.FullName())
}

// StreamID returns the proxy-assigned stream identifier for this
// stream
func (s *TargetStream) StreamID() uint64 {
	return s.streamID
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

// Send the data contained in 'req' to the client stream
func (s *TargetStream) Send(req *anypb.Any) error {
	// Frontload the check that the message is appropriate to the
	// method by unpacking the any here
	streamReq := s.serviceMethod.NewRequest()
	if err := req.UnmarshalTo(streamReq); err != nil {
		return err
	}
	ctx := s.grpcStream.Context()
	select {
	case s.reqChan <- streamReq:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run begins execution of the target stream
// All data received from target will be converted into ProxyReply
// messages for sending to a proxy client, including the final
// status of the target stream
func (s *TargetStream) Run(replyChan chan *pb.ProxyReply) {
	group, ctx := errgroup.WithContext(s.grpcStream.Context())
	group.Go(func() error {
		// read from the incoming request channel, and write
		// to the stream
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
						return s.grpcStream.CloseSend()
					}
					// otherwise, we're done
					return nil
				}
			}
			err := s.grpcStream.SendMsg(req)
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
				return err
			}
			// no error. If client streaming is not expected, then we're
			// done
			if !s.serviceMethod.ClientStreams() {
				return nil
			}
		}
	})
	// Receives messages from the server stream
	group.Go(func() error {
		for {
			msg := s.serviceMethod.NewReply()
			err := s.grpcStream.RecvMsg(msg)
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
	// Wait for final status from the errgroup, and translate it into
	// a server-close call
	err := group.Wait()
	log.Printf("finished: %s (%v)", s, err)
	var errStatus *pb.Status
	if err != nil {
		errStatus = convertStatus(status.New(codes.Internal, err.Error()))
	}
	reply := &pb.ProxyReply{
		Reply: &pb.ProxyReply_ServerClose{
			ServerClose: &pb.ServerClose{
				StreamIds: []uint64{s.streamID},
				Status:    errStatus,
			},
		},
	}
	replyChan <- reply
}

// NewTargetStream creates a new TargetStream for calling `method` on `target`
func NewTargetStream(ctx context.Context, target string, dialer TargetDialer, method *ServiceMethod) (*TargetStream, error) {
	ctx, cancel := context.WithCancel(ctx)
	conn, err := dialer.DialContext(ctx, target)
	if err != nil {
		return nil, err
	}
	clientStream, err := conn.NewStream(ctx, method.StreamDesc(), method.FullName())
	if err != nil {
		return nil, err
	}
	return &TargetStream{
		streamID:      rand.Uint64(),
		target:        target,
		serviceMethod: method,
		grpcStream:    clientStream,
		cancelFunc:    cancel,
		reqChan:       make(chan proto.Message),
	}, nil
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

	// The set of streams managed by this set
	streams map[uint64]*TargetStream

	// A WaitGroup used to track active streams
	wg sync.WaitGroup

	// A set of "target|nonce" strings, used to track previously
	// seen target/nonce pairs to prevent inadvertent re-use.
	noncePairs map[string]bool
}

// NewTargetStreamSet creates a TargetStreamSet which manages a set of related TargetStreams
func NewTargetStreamSet(serviceMethods map[string]*ServiceMethod, dialer TargetDialer) *TargetStreamSet {
	return &TargetStreamSet{
		serviceMethods: serviceMethods,
		targetDialer:   dialer,
		streams:        make(map[uint64]*TargetStream),
		noncePairs:     make(map[string]bool),
	}
}

// Add creates a new target stream for the given start stream request, and adds it to the set of streams
// managed by this set
//
// The result of stream creation, as well as any messages received from the created stream will be
// sent directly to 'replyChan'. If the stream was successfully started, its id will eventually be
// sent to 'doneChan' when all work has completed
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
	// TODO(jallie): authorization check for opening new stream goes here
	stream, err := NewTargetStream(ctx, req.GetTarget(), t.targetDialer, serviceMethod)
	if err != nil {
		reply.GetStartStreamReply().Reply = &pb.StartStreamReply_ErrorStatus{
			ErrorStatus: convertStatus(status.New(codes.Internal, err.Error())),
		}
		sendReply(reply)
		return nil
	}
	streamID := stream.StreamID()
	t.streams[streamID] = stream
	log.Println("new:", stream)
	reply.GetStartStreamReply().Reply = &pb.StartStreamReply_StreamId{
		StreamId: streamID,
	}
	t.noncePairs[targetNonce] = true
	t.wg.Add(1)
	go func() {
		stream.Run(replyChan)
		select {
		case doneChan <- streamID:
			// we notified caller of our status
		case <-ctx.Done():
			// or calling context is done
		}
		t.wg.Done()
	}()
	sendReply(reply)
	return nil
}

// Remove the stream corresponding to `streamid` from the
// stream set. Future references to this stream will return
// an error
func (t *TargetStreamSet) Remove(streamID uint64) {
	log.Println("removing:", streamID)
	delete(t.streams, streamID)
}

// Wait blocks until all TargetStreams associated with this
// stream set have completed
func (t *TargetStreamSet) Wait() {
	t.wg.Wait()
}

// ClientClose dispatches ClientClose requests to TargetStreams
// identified by ID in `req`
func (t *TargetStreamSet) ClientClose(req *pb.ClientClose) error {
	for _, id := range req.StreamIds {
		stream, ok := t.streams[id]
		if !ok {
			return status.Errorf(codes.InvalidArgument, "no such stream: %d", id)
		}
		log.Println("client close:", stream)
		stream.CloseSend()
	}
	return nil
}

// ClientCloseAll() issues ClientClose to all associated TargetStreams
func (t *TargetStreamSet) ClientCloseAll() {
	log.Println("client close all")
	for _, stream := range t.streams {
		log.Println("client close:", stream)
		stream.CloseSend()
	}
}

// ClientClose cancels TargetStreams identified by ID in `req`
func (t *TargetStreamSet) ClientCancel(req *pb.ClientCancel) error {
	for _, id := range req.StreamIds {
		stream, ok := t.streams[id]
		if !ok {
			return status.Errorf(codes.InvalidArgument, "no such stream: %d", id)
		}
		log.Println("cancel:", stream)
		stream.ClientCancel()
	}
	return nil
}

// Send dispatches new message data in `req` to the identified streams
func (t *TargetStreamSet) Send(req *pb.StreamData) error {
	// TODO(jallie): authorization check for sending to streams goes
	// here. StreamIds are collected, translated into targets, and
	// authorization check made before dispatch
	// TODO(jallie): this could also use parallel send, but since each
	// stream has it's own dispatching go-routine internally, this already
	// has some degree of underlying parallism
	for _, id := range req.StreamIds {
		stream, ok := t.streams[id]
		if !ok {
			return status.Errorf(codes.InvalidArgument, "no such stream: %d", id)
		}
		if err := stream.Send(req.Payload); err != nil {
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

func init() {
	rand.Seed(time.Now().UnixNano())
}
