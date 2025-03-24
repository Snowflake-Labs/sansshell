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

// Package server provides helpers for building and running a sansshell server.
package server

import (
	"context"
	"fmt"
	"github.com/Snowflake-Labs/sansshell/auth/rpcauthz"
	"io/fs"
	"net"
	"os"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/stats"

	"github.com/Snowflake-Labs/sansshell/services"
	"github.com/Snowflake-Labs/sansshell/telemetry"
)

// serveSetup describes everything needed to setup the RPC server.
// Documentation provided below in each WithXXX function.
type serveSetup struct {
	creds      credentials.TransportCredentials
	authorizer rpcauthz.RPCAuthorizer
	logger     logr.Logger
	// TODO: remove from here
	authzHooks         []rpcauthz.RPCAuthzHook
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	statsHandler       stats.Handler
	services           []func(*grpc.Server)
	onStartListeners   []func(*grpc.Server)
}

type Option interface {
	apply(context.Context, *serveSetup) error
}

type optionFunc func(context.Context, *serveSetup) error

func (o optionFunc) apply(ctx context.Context, s *serveSetup) error {
	return o(ctx, s)
}

// WithCredentials applies credentials to be used by the RPC server.
func WithCredentials(c credentials.TransportCredentials) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.creds = c
		return nil
	})
}

// WithInsecure specifies that transport security should be disabled.
func WithInsecure() Option {
	return WithCredentials(insecure.NewCredentials())
}

func WithRPCAuthorizer(a rpcauthz.RPCAuthorizer) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.authorizer = a
		return nil
	})
}

// WithLogger applies a logger that is used for all logging. A discard one is
// used if none is supplied.
func WithLogger(l logr.Logger) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.logger = l
		return nil
	})
}

// TODO: remote it from here
// WithAuthzHook adds an authz hook which is checked by the installed authorizer.
func WithAuthzHook(hook rpcauthz.RPCAuthzHook) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.authzHooks = append(s.authzHooks, hook)
		return nil
	})
}

// WithUnaryInterceptor adds an additional unary interceptor installed after telemetry and authz.
func WithUnaryInterceptor(unary grpc.UnaryServerInterceptor) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.unaryInterceptors = append(s.unaryInterceptors, unary)
		return nil
	})
}

// WithStreamInterceptor adds an additional stream interceptor installed after telemetry and authz.
func WithStreamInterceptor(stream grpc.StreamServerInterceptor) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.streamInterceptors = append(s.streamInterceptors, stream)
		return nil
	})
}

// WithRawServerOption allows one access to the RPC Server object. Generally this is done to add additional
// registration functions for RPC services to be done before starting the server.
func WithRawServerOption(s func(*grpc.Server)) Option {
	return optionFunc(func(_ context.Context, r *serveSetup) error {
		r.services = append(r.services, s)
		return nil
	})
}

// WithStatsHandler adds a stats handler for telemetry.
func WithStatsHandler(h stats.Handler) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.statsHandler = h
		return nil
	})
}

// WithOnStartListener adds a function to be called, in a goroutine, after the server
// has been created.
//
// This is useful for testing.
func WithOnStartListener(h func(*grpc.Server)) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.onStartListeners = append(s.onStartListeners, h)
		return nil
	})
}

// Serve starts and runs a server on the given hostport.
func Serve(hostport string, opts ...Option) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	return serveListener(lis, opts...)
}

// ServeUnix runs a server on a Unix socket. If the socket path exists, it will be removed beforehand.
func ServeUnix(socketPath string, socketConfigHook func(string) error, opts ...Option) error {
	// If the socket path exists, remove it, but error out if it is not a socket.
	if stat, err := os.Stat(socketPath); err == nil {
		if stat.Mode().Type() != fs.ModeSocket {
			return fmt.Errorf("Unix socket path exists and is not a socket: %s", socketPath)
		}
		if err := os.Remove(socketPath); err != nil {
			return fmt.Errorf("failed to remove existing Unix socket: %v", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat Unix socket: %v", err)
	}

	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on Unix socket: %v", err)
	}
	defer os.Remove(socketPath)

	if socketConfigHook != nil {
		if err := socketConfigHook(socketPath); err != nil {
			return fmt.Errorf("failed to configure Unix socket: %v", err)
		}
	}

	return serveListener(unixListener, opts...)
}

// serveListener wraps up BuildServer and runs a server for the given Listener object. It will automatically add
// an authz hook for HostNet based on the listener address. Additional hooks are passed along after this one.
//
// listener will be closed when this function returns.
func serveListener(listener net.Listener, opts ...Option) error {
	h := rpcauthz.HostNetHook(listener.Addr())
	opts = append(opts, WithAuthzHook(h))
	srv, err := BuildServer(opts...)
	if err != nil {
		return err
	}

	return srv.Serve(listener)
}

// BuildServer creates a gRPC server, attaches the RPC authz interceptor with supplied args and then
// registers all of the imported SansShell modules. Separating this from Serve
// primarily facilitates testing.
func BuildServer(opts ...Option) (*grpc.Server, error) {
	ctx := context.Background()
	ss := &serveSetup{
		logger: logr.Discard(),
	}
	for _, o := range opts {
		if err := o.apply(ctx, ss); err != nil {
			return nil, err
		}
	}
	if ss.authorizer == nil {
		return nil, fmt.Errorf("rpc authorizer was not provided")
	}
	ss.authorizer.AppendHooks(ss.authzHooks...)

	unary := ss.unaryInterceptors
	unary = append(unary,
		// Execute log interceptor after other interceptors so that metadata gets logged
		telemetry.UnaryServerLogInterceptor(ss.logger),
		// Execute authz after logger is setup
		ss.authorizer.Authorize,
	)
	streaming := ss.streamInterceptors
	streaming = append(streaming,
		// Execute log interceptor after other interceptors so that metadata gets logged
		telemetry.StreamServerLogInterceptor(ss.logger),
		// Execute authz after logger is setup
		ss.authorizer.AuthorizeStream,
	)
	serverOpts := []grpc.ServerOption{
		grpc.Creds(ss.creds),
		// NB: the order of chained interceptors is meaningful.
		// The first interceptor is outermost, and the final interceptor will wrap the real handler.
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(streaming...),
	}
	if ss.statsHandler != nil {
		serverOpts = append(serverOpts, grpc.StatsHandler(ss.statsHandler))
	}

	s := grpc.NewServer(serverOpts...)

	// Always register anything which is in the list as it was pulled in there
	// via import and intended.
	for _, sansShellService := range services.ListServices() {
		sansShellService.Register(s)
	}

	// Now loop over any other registered and call them.
	for _, srv := range ss.services {
		srv(s)
	}

	// Call any on-start listeners that were registered.
	for _, listener := range ss.onStartListeners {
		go listener(s)
	}

	return s, nil
}
