/* Copyright (c) 2026 Snowflake Inc. All rights reserved.

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
	"net"
	"slices"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// NewCredentialsDialer creates a TargetDialer that connects to targets with creds and
// records the identity (address and TLS certificate) each target presents during the
// handshake.
//
// Policies see the target's identity when they authorize a request. gRPC only exposes
// that identity through an open stream, and request metadata can only be sent when a
// stream opens, so with NewDialer the proxy has to open the stream before it can
// authorize anything. Recording the identity at the handshake lets the proxy authorize
// the first request first and then open the stream with any metadata the policy set
// (see rpcauth.RPCAuthInput.TargetMetadata). Otherwise it behaves like NewDialer with
// grpc.WithTransportCredentials(creds).
func NewCredentialsDialer(creds credentials.TransportCredentials, opts ...grpc.DialOption) TargetDialer {
	return &credentialsDialer{optionsDialer: optionsDialer{opts: opts}, creds: creds}
}

type credentialsDialer struct {
	optionsDialer
	creds credentials.TransportCredentials
}

func (d *credentialsDialer) DialContext(ctx context.Context, target string, dialOpts ...grpc.DialOption) (ClientConnCloser, error) {
	recorder := &identityRecorder{TransportCredentials: d.creds, peer: &atomic.Pointer[peer.Peer]{}}
	conn, err := d.optionsDialer.DialContext(ctx, target, slices.Concat(dialOpts, []grpc.DialOption{grpc.WithTransportCredentials(recorder)})...)
	if err != nil {
		return nil, err
	}
	return &identifiedConn{ClientConn: conn.(*grpc.ClientConn), peer: recorder.peer}, nil
}

type identifiedTarget interface {
	targetPeer(ctx context.Context) (*peer.Peer, error)
}

type identifiedConn struct {
	*grpc.ClientConn
	peer *atomic.Pointer[peer.Peer]
}

func (c *identifiedConn) targetPeer(ctx context.Context) (*peer.Peer, error) {
	c.Connect()
	for {
		state := c.GetState()
		switch state {
		case connectivity.Ready:
			if p := c.peer.Load(); p != nil {
				return p, nil
			}
			return nil, status.Error(codes.Internal, "connection is ready but the target identity was not recorded")
		case connectivity.TransientFailure:
			if err := c.connectionError(ctx); err != nil {
				return nil, err
			}
			continue
		case connectivity.Shutdown:
			return nil, status.Error(codes.Unavailable, "connection to target is shut down")
		}
		if !c.WaitForStateChange(ctx, state) {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	}
}

// gRPC does not expose why a connection failed. A fail-fast stream on a failed
// connection returns that reason without reaching the target. If the connection
// recovered in the meantime, the stream goes to a method no server implements and is
// cancelled before any message is sent.
func (c *identifiedConn) connectionError(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, err := c.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, "/sansshell.proxy.ConnectionCheck/Unavailable")
	return err
}

type identityRecorder struct {
	credentials.TransportCredentials
	peer *atomic.Pointer[peer.Peer]
}

func (r *identityRecorder) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, info, err := r.TransportCredentials.ClientHandshake(ctx, authority, rawConn)
	if err != nil {
		return nil, nil, err
	}
	r.peer.Store(&peer.Peer{Addr: conn.RemoteAddr(), AuthInfo: info})
	return conn, info, nil
}

// gRPC may clone the credentials, and the clone must keep reporting to the same
// connection.
func (r *identityRecorder) Clone() credentials.TransportCredentials {
	return &identityRecorder{TransportCredentials: r.TransportCredentials.Clone(), peer: r.peer}
}
