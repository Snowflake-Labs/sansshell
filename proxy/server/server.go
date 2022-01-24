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

// package server provides the server-side implementation of the
// sansshell proxy server
package server

import (
	"context"
	"fmt"
	"io"
	"log"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	pb "github.com/Snowflake-Labs/sansshell/proxy"
)

// A TargetDialer is used by the proxy server to make connections
// to requested targets
// It encapsulates the various low-level details of making target
// connections (such as client credentials, deadlines, etc) which
// the proxy can use without needing to understand them.
type TargetDialer interface {
	DialContext(ctx context.Context, target string) (grpc.ClientConnInterface, error)
}

// an optionsDialer implements TargetDialer using native grpc.Dial
type optionsDialer struct {
	opts []grpc.DialOption
}

// See TargetDialer.DialContext
func (o *optionsDialer) DialContext(ctx context.Context, target string) (grpc.ClientConnInterface, error) {
	return grpc.DialContext(ctx, target, o.opts...)
}

// NewDialer creates a new TargetDialer that uses grpc.Dial with the
// supplied DialOptions
func NewDialer(opts ...grpc.DialOption) TargetDialer {
	return &optionsDialer{opts: opts}
}

// Server implements proxy.ProxyServer
type Server struct {
	// A map of /Package.Service/Method => ServiceMethod
	serviceMap map[string]*ServiceMethod

	// A dialer for making proxy -> target connections
	dialer TargetDialer

	// A policy authorizer, for authorizing proxy -> target requests
	authorizer *rpcauth.Authorizer
}

// Register registers this server with the given ServiceRegistrar
// (typically a grpc.Server)
func (s *Server) Register(sr grpc.ServiceRegistrar) {
	pb.RegisterProxyServer(sr, s)
}

// New creates a new Server which will use the supplied TargetDialer
// for opening new target connections, and the global protobuf
// registry to resolve service methods
// The supplied authorizer is used to authorize requests made
// to targets.
func New(dialer TargetDialer, authorizer *rpcauth.Authorizer) *Server {
	return NewWithServiceMap(dialer, authorizer, LoadGlobalServiceMap())
}

// NewWithServiceMap create a new Server using the supplied TargetDialer
// and service map.
// The supplied authorizer is used to authorize requests made
// to targets.
func NewWithServiceMap(dialer TargetDialer, authorizer *rpcauth.Authorizer, serviceMap map[string]*ServiceMethod) *Server {
	return &Server{
		serviceMap: serviceMap,
		dialer:     dialer,
		authorizer: authorizer,
	}
}

// Proxy implements ProxyServer.Proxy to provide a single bidirectional
// stream which manages requests to a set of one or more backend
// target servers
func (s *Server) Proxy(stream pb.Proxy_ProxyServer) error {
	requestChan := make(chan *pb.ProxyRequest)
	replyChan := make(chan *pb.ProxyReply)

	group, ctx := errgroup.WithContext(stream.Context())

	// create a new TargetStreamSet to manage the target streams
	// associated with this proxy connection
	streamSet := NewTargetStreamSet(s.serviceMap, s.dialer, s.authorizer)

	// A single go-routine for handling all sends to the reply
	// channel
	// While a stream can be safely used for both send and receive
	// simultaneously, it is not safe for multiple goroutines
	// to call "Send" on the same stream
	group.Go(func() error {
		return send(replyChan, stream)
	})

	// A single go-routine for receiving all incoming requests from
	// the client
	// While a stream can be safely used for both send and receive
	// simultaneously, it is not safe for multiple goroutines
	// to call "Recv" on the same stream
	group.Go(func() error {
		// This double-dispatch is necessary because Recv() will block
		// until the proxy stream itself is cancelled.
		// If dispatch has failed, we need to exit, even though Recv
		// is still active
		// In this case, any error returned from Recv is safe to discard
		// since the errgroup will already contain the correct status
		// to return to the client.
		errChan := make(chan error)
		go func() {
			err := receive(ctx, stream, requestChan)
			select {
			case errChan <- err:
			default:
				// our parent has exited
			}
			close(errChan)
		}()
		select {
		case err := <-errChan:
			return err
		case <-ctx.Done():
			log.Printf("Checking context: %v", ctx.Err())
			return ctx.Err()
		}
	})

	// This dispatching goroutine manages request dispatch to a set of
	// active target streams
	group.Go(func() error {
		// when we finish dispatching, we're done, and will send no further
		// messages to the reply channel
		// This will signal the Send goroutine to exit
		defer close(replyChan)

		// Create a derived, cancellable context that we can use to tear
		// down all streams in case of error.
		ctx, cancel := context.WithCancel(ctx)

		// Invoke dispatch to handle incoming requests
		err := dispatch(ctx, requestChan, replyChan, streamSet)

		// If dispatch returned with an error, we can cancel all
		// running streams by cancelling their context.
		if err != nil {
			cancel()
		}

		// Wait for running streams to exit.
		streamSet.Wait()

		cancel()
		return err
	})

	// Final RPC status is the status of the waitgroup
	err := group.Wait()

	if err != nil {
		return err
	}
	return nil
}

// send relays messages from `replyChan` to the provided stream
func send(replyChan chan *pb.ProxyReply, stream pb.Proxy_ProxyServer) error {
	for msg := range replyChan {
		if err := stream.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

// receive relays incoming messages received from the provided stream to `requestChan`
// until EOF (or other error) is received from the stream, or the supplied context is
// done
func receive(ctx context.Context, stream pb.Proxy_ProxyServer, requestChan chan *pb.ProxyRequest) error {
	// Close 'requestChan' when receive returns, since we will
	// never receive any additional messages from the client
	// This can be used by the dispatching goroutine as a single
	// to CloseSend on the target streams
	defer close(requestChan)
	for {
		// Receive from the client stream
		// This will block, but can return early
		// if the stream context is cancelled
		req, err := stream.Recv()
		log.Printf("err from Recv: %+v", err)
		if err == io.EOF {
			// On the server, io.EOF indicates that the
			// client has issued as CloseSend(), and will
			// issue no further requests
			// Returning here will close requestChan, which
			// we can use as a signal to propogate the CloseSend
			// to all running target streams
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case requestChan <- req:
		case <-ctx.Done():
			log.Printf("Context error: %+v", ctx.Err())
			return ctx.Err()
		}
	}
}

// dispatch manages incoming requests from `requestChan` by routing them to the supplied stream set
func dispatch(ctx context.Context, requestChan chan *pb.ProxyRequest, replyChan chan *pb.ProxyReply, streamSet *TargetStreamSet) error {
	// Channel to track streams that have completed and should
	// be removed from the stream set
	doneChan := make(chan uint64)

	for {
		select {
		case <-ctx.Done():
			// Our context has ended. This should propogate automtically
			// to all target streams
			return ctx.Err()
		case closedStream := <-doneChan:
			// A stream has closed, and sent its final ServerClose status
			// Remove it from the active streams list. Further messages
			// received with this stream ID will return an error to the
			// client.
			streamSet.Remove(closedStream)
		case req, ok := <-requestChan:
			if !ok {
				// The request channel has been closed
				// This could occur if the proxy client executes
				// a CloseSend(), or Send/Recv() from the client
				// stream has failed with an error
				// In the latter case, the context cancellation
				// should eventually propagate to the target
				// streams, and cause them to finish
				// In either case, we should let the target streams
				// know that no further requests will be arriving
				streamSet.ClientCloseAll()
				return nil
			}
			// We have a new request
			switch req.Request.(type) {
			case *pb.ProxyRequest_StartStream:
				if err := streamSet.Add(ctx, req.GetStartStream(), replyChan, doneChan); err != nil {
					return err
				}
			case *pb.ProxyRequest_StreamData:
				if err := streamSet.Send(ctx, req.GetStreamData()); err != nil {
					return err
				}
			case *pb.ProxyRequest_ClientCancel:
				if err := streamSet.ClientCancel(req.GetClientCancel()); err != nil {
					return err
				}
			case *pb.ProxyRequest_ClientClose:
				if err := streamSet.ClientClose(req.GetClientClose()); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unhandled request type %T", req.Request)
			}
		}
	}
}
