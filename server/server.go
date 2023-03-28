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
	"net"
	"sync"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/services"
	"github.com/Snowflake-Labs/sansshell/telemetry"
)

var (
	// Provide as a var so tests can cancel the server.
	srv *grpc.Server
	mu  sync.Mutex
)

// serveSetup describes everything needed to setup the RPC server.
// Documentation provided below in each WithXXX function.
type serveSetup struct {
	creds              credentials.TransportCredentials
	policy             string
	logger             logr.Logger
	authzHooks         []rpcauth.RPCAuthzHook
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	services           []func(*grpc.Server)
}

type Option interface {
	apply(*serveSetup) error
}

type optionFunc func(*serveSetup) error

func (o optionFunc) apply(s *serveSetup) error {
	return o(s)
}

// WithCredentials applies credentials to be used by the RPC server.
func WithCredentials(c credentials.TransportCredentials) Option {
	return optionFunc(func(s *serveSetup) error {
		s.creds = c
		return nil
	})
}

// WithPolicy applies an OPA policy used against incoming RPC requests.
func WithPolicy(policy string) Option {
	return optionFunc(func(s *serveSetup) error {
		s.policy = policy
		return nil
	})
}

// WithLogger applies a logger that is used for all logging. A discard one is
// used if none is supplied.
func WithLogger(l logr.Logger) Option {
	return optionFunc(func(s *serveSetup) error {
		s.logger = l
		return nil
	})
}

// WithAuthzHook adds an authz hook which is checked by the installed authorizer.
func WithAuthzHook(hook rpcauth.RPCAuthzHook) Option {
	return optionFunc(func(s *serveSetup) error {
		s.authzHooks = append(s.authzHooks, hook)
		return nil
	})
}

// WithUnaryInterceptor adds an additional unary interceptor installed after telemetry and authz.
func WithUnaryInterceptor(unary grpc.UnaryServerInterceptor) Option {
	return optionFunc(func(s *serveSetup) error {
		s.unaryInterceptors = append(s.unaryInterceptors, unary)
		return nil
	})
}

// WithStreamInterceptor adds an additional stream interceptor installed after telemetry and authz.
func WithStreamInterceptor(stream grpc.StreamServerInterceptor) Option {
	return optionFunc(func(s *serveSetup) error {
		s.streamInterceptors = append(s.streamInterceptors, stream)
		return nil
	})
}

// WithRawServerOption allows one access to the RPC Server object. Generally this is done to add additional
// registration functions for RPC services to be done before starting the server.
func WithRawServerOption(s func(*grpc.Server)) Option {
	return optionFunc(func(r *serveSetup) error {
		r.services = append(r.services, s)
		return nil
	})
}

// Serve wraps up BuildServer in a succinct API for callers passing along various parameters. It will automatically add
// an authz hook for HostNet based on the listener address. Additional hooks are passed along after this one.
func Serve(hostport string, opts ...Option) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	mu.Lock()
	h := rpcauth.HostNetHook(lis.Addr())
	opts = append(opts, WithAuthzHook(h))
	srv, err = BuildServer(opts...)
	mu.Unlock()
	if err != nil {
		return err
	}

	return srv.Serve(lis)
}

// Test helper to get at srv
func getSrv() *grpc.Server {
	mu.Lock()
	defer mu.Unlock()
	return srv
}

// BuildServer creates a gRPC server, attaches the OPA policy interceptor with supplied args and then
// registers all of the imported SansShell modules. Separating this from Serve
// primarily facilitates testing.
func BuildServer(opts ...Option) (*grpc.Server, error) {
	ss := &serveSetup{
		logger: logr.Discard(),
	}
	for _, o := range opts {
		if err := o.apply(ss); err != nil {
			return nil, err
		}
	}

	authz, err := rpcauth.NewWithPolicy(context.Background(), ss.policy, ss.authzHooks...)
	if err != nil {
		return nil, err
	}

	unary := ss.unaryInterceptors
	unary = append(unary,
		telemetry.UnaryServerLogInterceptor(ss.logger),
		authz.Authorize,
	)
	streaming := ss.streamInterceptors
	streaming = append(streaming,
		telemetry.StreamServerLogInterceptor(ss.logger),
		authz.AuthorizeStream,
	)
	serverOpts := []grpc.ServerOption{
		grpc.Creds(ss.creds),
		// NB: the order of chained interceptors is meaningful.
		// The first interceptor is outermost, and the final interceptor will wrap the real handler.
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(streaming...),
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

	return s, nil
}
