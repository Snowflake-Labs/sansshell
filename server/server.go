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

// Serve wraps up BuildServer in a succinct API for callers passing along various parameters. It will automatically add
// an authz hook for HostNet based on the listener address. Additional hooks are passed along after this one.
func Serve(hostport string, c credentials.TransportCredentials, policy string, logger logr.Logger, authzHooks ...rpcauth.RPCAuthzHook) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	mu.Lock()
	h := []rpcauth.RPCAuthzHook{rpcauth.HostNetHook(lis.Addr())}
	h = append(h, authzHooks...)

	srv, err = BuildServer(c, policy, logger, h...)
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
func BuildServer(c credentials.TransportCredentials, policy string, logger logr.Logger, authzHooks ...rpcauth.RPCAuthzHook) (*grpc.Server, error) {
	authz, err := rpcauth.NewWithPolicy(context.Background(), policy, authzHooks...)
	if err != nil {
		return nil, err
	}
	opts := []grpc.ServerOption{
		grpc.Creds(c),
		// NB: the order of chained interceptors is meaningful.
		// The first interceptor is outermost, and the final interceptor will wrap the real handler.
		grpc.ChainUnaryInterceptor(telemetry.UnaryServerLogInterceptor(logger), authz.Authorize),
		grpc.ChainStreamInterceptor(telemetry.StreamServerLogInterceptor(logger), authz.AuthorizeStream),
	}
	s := grpc.NewServer(opts...)
	for _, sansShellService := range services.ListServices() {
		sansShellService.Register(s)
	}
	return s, nil
}
