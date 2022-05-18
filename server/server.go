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
	channelz "google.golang.org/grpc/channelz/service"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/services"
	"github.com/Snowflake-Labs/sansshell/telemetry"
)

var (
	// Provide as a var so tests can cancel the server.
	srv *grpc.Server
	mu  sync.Mutex
)

type ServeSetup struct {
	Creds              credentials.TransportCredentials
	Policy             string
	Logger             logr.Logger
	AuthzHooks         []rpcauth.RPCAuthzHook
	UnaryInterceptors  []grpc.UnaryServerInterceptor
	StreamInterceptors []grpc.StreamServerInterceptor
}

// Serve wraps up BuildServer in a succinct API for callers passing along various parameters. It will automatically add
// an authz hook for HostNet based on the listener address. Additional hooks are passed along after this one.
func Serve(hostport string, setup ServeSetup) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	mu.Lock()
	h := []rpcauth.RPCAuthzHook{rpcauth.HostNetHook(lis.Addr())}
	h = append(h, setup.AuthzHooks...)

	setup.AuthzHooks = h
	srv, err = BuildServer(setup)
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
func BuildServer(setup ServeSetup) (*grpc.Server, error) {
	authz, err := rpcauth.NewWithPolicy(context.Background(), setup.Policy, setup.AuthzHooks...)
	if err != nil {
		return nil, err
	}

	unary := []grpc.UnaryServerInterceptor{
		telemetry.UnaryServerLogInterceptor(setup.Logger),
		authz.Authorize,
	}
	unary = append(unary, setup.UnaryInterceptors...)
	streaming := []grpc.StreamServerInterceptor{
		telemetry.StreamServerLogInterceptor(setup.Logger),
		authz.AuthorizeStream,
	}
	streaming = append(streaming, setup.StreamInterceptors...)
	opts := []grpc.ServerOption{
		grpc.Creds(setup.Creds),
		// NB: the order of chained interceptors is meaningful.
		// The first interceptor is outermost, and the final interceptor will wrap the real handler.
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(streaming...),
	}
	s := grpc.NewServer(opts...)
	reflection.Register(s)
	channelz.RegisterChannelzServiceToServer(s)

	for _, sansShellService := range services.ListServices() {
		sansShellService.Register(s)
	}
	return s, nil
}
