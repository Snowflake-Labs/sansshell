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

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/stats"

	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/services"
	"github.com/Snowflake-Labs/sansshell/telemetry"
)

// serveSetup describes everything needed to setup the RPC server.
// Documentation provided below in each WithXXX function.
type serveSetup struct {
	creds              credentials.TransportCredentials
	policy             *opa.AuthzPolicy
	logger             logr.Logger
	authzHooks         []rpcauth.RPCAuthzHook
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

// WithPolicy applies an OPA policy used against incoming RPC requests.
func WithPolicy(policy string) Option {
	return optionFunc(func(ctx context.Context, s *serveSetup) error {
		p, err := opa.NewAuthzPolicy(ctx, policy)
		if err != nil {
			return err
		}
		s.policy = p
		return nil
	})
}

// WithParsedPolicy applies an already-parsed OPA policy used against incoming RPC requests.
func WithParsedPolicy(policy *opa.AuthzPolicy) Option {
	return optionFunc(func(_ context.Context, s *serveSetup) error {
		s.policy = policy
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

// WithAuthzHook adds an authz hook which is checked by the installed authorizer.
func WithAuthzHook(hook rpcauth.RPCAuthzHook) Option {
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

// Serve wraps up BuildServer in a succinct API for callers passing along various parameters. It will automatically add
// an authz hook for HostNet based on the listener address. Additional hooks are passed along after this one.
func Serve(hostport string, opts ...Option) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	h := rpcauth.HostNetHook(lis.Addr())
	opts = append(opts, WithAuthzHook(h))
	srv, err := BuildServer(opts...)
	if err != nil {
		return err
	}

	return srv.Serve(lis)
}

// BuildServer creates a gRPC server, attaches the OPA policy interceptor with supplied args and then
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
	if ss.policy == nil {
		return nil, fmt.Errorf("policy was not provided")
	}

	authz := rpcauth.New(ss.policy, ss.authzHooks...)

	unary := ss.unaryInterceptors
	unary = append(unary,
		// Execute log interceptor after other interceptors so that metadata gets logged
		telemetry.UnaryServerLogInterceptor(ss.logger),
		// Execute authz after logger is setup
		authz.Authorize,
	)
	streaming := ss.streamInterceptors
	streaming = append(streaming,
		// Execute log interceptor after other interceptors so that metadata gets logged
		telemetry.StreamServerLogInterceptor(ss.logger),
		// Execute authz after logger is setup
		authz.AuthorizeStream,
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
