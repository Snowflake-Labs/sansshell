package server

import (
	"context"
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
// the proxy and a a target sansshell server.
type TargetStream struct {
	// The unique indentifier of this stream.
	streamID uint64

	// The address of the target server, as given in StartStream
	target string

	// The ServiceMethod spec.
	serviceMethod *ServiceMethod

	// The underlying grpc.ClientStream to the target server.
	grpcStream grpc.ClientStream

	// A cancel function that can be used to request early cancellation
	// of the stream context.
	cancelFunc context.CancelFunc

	// A 'once' guarding CloseSend to allow it to be called idempotently.
	closeOnce sync.Once

	// the (internal) channel used to manage incoming requests.
	reqChan chan proto.Message
}

// StreamID returns the proxy-assigned stream identifier for this
// stream.
func (s *TargetStream) StreamID() uint64 {
	return s.streamID
}

// CloseSend is used to indicate that no more client requests
// will be sent to this stream.
func (s *TargetStream) CloseSend() {
	s.closeOnce.Do(func() {
		close(s.reqChan)
	})
}

// ClientCancel requests cancellation of the stream.
func (s *TargetStream) ClientCancel() {
	s.cancelFunc()
}

// Send the data contained in 'req' to the client stream.
func (s *TargetStream) Send(req *anypb.Any) error {
	// Frontload the check that the message is appropriate to the
	// method by unpacking the any here.
	streamReq := s.serviceMethod.NewRequest()
	if err := req.UnmarshalTo(streamReq); err != nil {
		return err
	}
	log.Println("sending stream reqeust: ", streamReq)
	ctx := s.grpcStream.Context()
	select {
	case s.reqChan <- streamReq:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run begins execution of the target stream.
// All data received from target will be converted into ProxyReply
// messages for sending to a proxy client, including the final
// status of the target stream.
func (s *TargetStream) Run(replyChan chan *pb.ProxyReply) {
	group, ctx := errgroup.WithContext(s.grpcStream.Context())
	group.Go(func() error {
		// read from the incoming request channel, and writes
		// to the stream.
		for req := range s.reqChan {
			err := s.grpcStream.SendMsg(req)
			// if this returns an EOF, then the final status
			// will be returned via a call to RecvMsg, and we
			// should not return an error here, since that
			// would cancel the errgroup early. Instead, we
			// can return nil, and the error will be picked
			// up by the receiving goroutine.
			if err == io.EOF {
				return nil
			}
			// Otherwise, this is the 'final' error. The underlying
			// stream will be torn down automatically, but we
			// can return the error here, where it will be returned
			// by the errgroup, and sent in the ServerClose.
			if err != nil {
				return err
			}
			// no error. If client streaming is not expected, then we're
			// done.
			if !s.serviceMethod.ClientStreams() {
				return nil
			}
		}
		// reqChan was closed, meaning no more messages are incoming.
		// if this was a client stream, we issue a half-close
		if s.serviceMethod.ClientStreams() {
			s.grpcStream.CloseSend()
		}
		return nil
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
			log.Println("recieved message from target")
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
			select {
			case replyChan <- reply:
				// sent: todo() log.Debug?
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	// Wait for final status from the errgroup, and translate it into
	// a server-close call.
	err := group.Wait()
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
	select {
	case replyChan <- reply:
		// done
	case <-s.grpcStream.Context().Done():
		// done
	}
}

// NewTargetStream creates a new TargetStream for calling `method` on `target`.
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
// synchronization.
type TargetStreamSet struct {
	// A service method map used to resolve incoming stream requests to service methods.
	serviceMethods map[string]*ServiceMethod

	// A TargetDialer for initiating target connections.
	targetDialer TargetDialer

	// The set of streams managed by this set.
	streams map[uint64]*TargetStream

	// A WaitGroup used to track active streams.
	wg sync.WaitGroup
}

func NewTargetStreamSet(serviceMethods map[string]*ServiceMethod, dialer TargetDialer) *TargetStreamSet {
	return &TargetStreamSet{
		serviceMethods: serviceMethods,
		targetDialer:   dialer,
		streams:        make(map[uint64]*TargetStream),
	}
}

// Add creates a new target stream for the given start stream request, and adds it to the set of streams
// managed by this set.
// The result of stream creation, as well as any messages received from the created stream will be
// sent directly to 'replyChan'.
// If the stream was successfully started, it's id will eventually be sent to 'doneChan' when all work
// has completed.
func (t *TargetStreamSet) Add(ctx context.Context, req *pb.StartStream, replyChan chan *pb.ProxyReply, doneChan chan uint64) {
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
		return
	}
	stream, err := NewTargetStream(ctx, req.GetTarget(), t.targetDialer, serviceMethod)
	if err != nil {
		reply.GetStartStreamReply().Reply = &pb.StartStreamReply_ErrorStatus{
			ErrorStatus: convertStatus(status.New(codes.Internal, err.Error())),
		}
		sendReply(reply)
		return
	}
	streamID := stream.StreamID()
	t.streams[streamID] = stream
	reply.GetStartStreamReply().Reply = &pb.StartStreamReply_StreamId{
		StreamId: streamID,
	}
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
}

// Remove the stream corresponding to `streamid` from the
// stream set. Future references to this stream will return
// an error.
func (t *TargetStreamSet) Remove(streamID uint64) {
	delete(t.streams, streamID)
}

// Wait blocks until all managed streams have finished
// execution.
func (t *TargetStreamSet) Wait() {
	t.wg.Wait()
}

func (t *TargetStreamSet) ClientClose(req *pb.ClientClose) error {
	for _, id := range req.StreamIds {
		stream, ok := t.streams[id]
		if !ok {
			return status.Errorf(codes.InvalidArgument, "no such stream: %d", id)
		}
		stream.CloseSend()
	}
	return nil
}

func (t *TargetStreamSet) ClientCloseAll() {
	for _, stream := range t.streams {
		stream.CloseSend()
	}
}

func (t *TargetStreamSet) ClientCancel(req *pb.ClientCancel) error {
	for _, id := range req.StreamIds {
		stream, ok := t.streams[id]
		if !ok {
			return status.Errorf(codes.InvalidArgument, "no such stream: %d", id)
		}
		stream.ClientCancel()
	}
	return nil
}

func (t *TargetStreamSet) Send(req *pb.StreamData) error {
	// TODO(jallie): this could also use parallel send, but since each
	// stream has it's own dispatching go-routine internally, this already
	// has some degree of underlying parallism.
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

func init() {
	rand.Seed(time.Now().UnixNano())
}
