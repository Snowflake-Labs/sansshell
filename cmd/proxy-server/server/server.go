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

// Package server provides functionality so that other uses of sansshell can provide their
// own main.go without having to cargo-cult everything across for common use cases. i.e. simply
// adding your own authz hooks but using the standard modules. Or adding additional modules that
// are locally defined.
package server

import (
	"context"
	"net"
	"os"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/server"
	ss "github.com/Snowflake-Labs/sansshell/services/sansshell/server"
	"github.com/Snowflake-Labs/sansshell/telemetry"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RunState encapsulates all of the variable state needed
// to run a proxy server.
type RunState struct {
	// Logger is used for all logging.
	Logger logr.Logger
	// Policy is an OPA policy for determining authz decisions.
	Policy string
	// ClientPolicy is an optional OPA policy for determining outbound decisions.
	ClientPolicy string
	// CredSource is a registered credential source with the mtls package.
	CredSource string
	// Hostport is the host:port to run the server.
	Hostport string
	// Justification if true requires justification to be set in the
	// incoming RPC context Metadata (to the key defined in the telemetry package).
	Justification bool
	// JustificationFunc will be called if Justication is true and a justification
	// entry is found. The supplied function can then do any validation it wants
	// in order to ensure it's compliant.
	JustificationFunc func(string) error
	// UnaryInterceptors are any additional interceptors to be added to unary RPCs
	// served from this instance. They will be added after logging and authz checks.
	UnaryInterceptors []grpc.UnaryServerInterceptor
	// UnaryClientInterceptors are any additional interceptors to be added to unary RPCs
	// requested from this instance. They will be added after logging and authz checks.
	UnaryClientInterceptors []grpc.UnaryClientInterceptor
	// StreamInterceptors are any additional interceptors to be added to streaming RPCs
	// served from this instance. They will be added after logging and authz checks.
	StreamInterceptors []grpc.StreamServerInterceptor
	// StreamClientInterceptors are any additional interceptors to be added to streaming RPCs
	// requested from this instance. They will be added after logging and authz checks.
	StreamClientInterceptors []grpc.StreamClientInterceptor
}

// Run takes the given context and RunState along with any authz hooks and starts up a sansshell proxy server
// using the flags above to provide credentials. An address hook (based on the remote host) with always be added.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, rs RunState, hooks ...rpcauth.RPCAuthzHook) {
	serverCreds, err := mtls.LoadServerCredentials(ctx, rs.CredSource)
	if err != nil {
		rs.Logger.Error(err, "mtls.LoadServerCredentials", "credsource", rs.CredSource)
		os.Exit(1)
	}
	clientCreds, err := mtls.LoadClientCredentials(ctx, rs.CredSource)
	if err != nil {
		rs.Logger.Error(err, "mtls.LoadClientCredentials", "credsource", rs.CredSource)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", rs.Hostport)
	if err != nil {
		rs.Logger.Error(err, "net.Listen", "hostport", rs.Hostport)
		os.Exit(1)
	}
	rs.Logger.Info("listening", "hostport", rs.Hostport)

	addressHook := rpcauth.HookIf(rpcauth.HostNetHook(lis.Addr()), func(input *rpcauth.RPCAuthInput) bool {
		return input.Host == nil || input.Host.Net == nil
	})
	justificationHook := rpcauth.HookIf(rpcauth.JustificationHook(rs.JustificationFunc), func(input *rpcauth.RPCAuthInput) bool {
		return rs.Justification
	})

	h := []rpcauth.RPCAuthzHook{addressHook, justificationHook}
	h = append(h, hooks...)
	authz, err := rpcauth.NewWithPolicy(ctx, rs.Policy, h...)
	if err != nil {
		rs.Logger.Error(err, "rpcauth.NewWithPolicy")
		os.Exit(1)
	}

	var clientAuthz *rpcauth.Authorizer
	if rs.ClientPolicy != "" {
		clientAuthz, err = rpcauth.NewWithPolicy(ctx, rs.ClientPolicy)
		if err != nil {
			rs.Logger.Error(err, "client rpcauth.NewWithPolicy")
		}
	}

	// We always have the logger but might need to chain if we're also doing client outbound OPA checks.
	unaryClient := []grpc.UnaryClientInterceptor{
		telemetry.UnaryClientLogInterceptor(rs.Logger),
	}
	streamClient := []grpc.StreamClientInterceptor{
		telemetry.StreamClientLogInterceptor(rs.Logger),
	}
	if clientAuthz != nil {
		unaryClient = append(unaryClient, clientAuthz.AuthorizeClient)
		streamClient = append(streamClient, clientAuthz.AuthorizeClientStream)
	}
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(clientCreds),
		grpc.WithChainUnaryInterceptor(unaryClient...),
		grpc.WithChainStreamInterceptor(streamClient...),
	}
	targetDialer := server.NewDialer(dialOpts...)

	svcMap := server.LoadGlobalServiceMap()
	rs.Logger.Info("loaded service map", "serviceMap", svcMap)
	server := server.New(targetDialer, authz)

	// Even though the proxy RPC is streaming we have unary RPCs (logging, reflection) we
	// also need to properly auth and log.
	unaryServer := []grpc.UnaryServerInterceptor{
		telemetry.UnaryServerLogInterceptor(rs.Logger),
		authz.Authorize,
	}
	unaryServer = append(unaryServer, rs.UnaryInterceptors...)
	streamServer := []grpc.StreamServerInterceptor{
		telemetry.StreamServerLogInterceptor(rs.Logger),
		authz.AuthorizeStream,
	}
	streamServer = append(streamServer, rs.StreamInterceptors...)
	serverOpts := []grpc.ServerOption{
		grpc.Creds(serverCreds),
		grpc.ChainUnaryInterceptor(unaryServer...),
		grpc.ChainStreamInterceptor(streamServer...),
	}
	g := grpc.NewServer(serverOpts...)

	server.Register(g)
	reflection.Register(g)
	// Create a an instance of logging for the proxy server itself.
	s := &ss.Server{}
	s.Register(g)
	rs.Logger.Info("initialized proxy service", "credsource", rs.CredSource)
	rs.Logger.Info("serving..")

	if err := g.Serve(lis); err != nil {
		rs.Logger.Error(err, "grpcserver.Serve()")
		os.Exit(1)
	}
}
