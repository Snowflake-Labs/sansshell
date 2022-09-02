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
	"crypto/tls"
	"net"
	"os"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/server"
	"github.com/Snowflake-Labs/sansshell/telemetry"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

// runState encapsulates all of the variable state needed
// to run a proxy server. Documentation provided below in each
// WithXXX function.
type runState struct {
	logger                   logr.Logger
	policy                   string
	clientPolicy             string
	credSource               string
	tlsConfig                *tls.Config
	hostport                 string
	justification            bool
	justificationFunc        func(string) error
	unaryInterceptors        []grpc.UnaryServerInterceptor
	unaryClientInterceptors  []grpc.UnaryClientInterceptor
	streamInterceptors       []grpc.StreamServerInterceptor
	streamClientInterceptors []grpc.StreamClientInterceptor
	authzHooks               []rpcauth.RPCAuthzHook
	services                 []func(*grpc.Server)
}

type Option interface {
	apply(*runState) error
}

type optionFunc func(*runState) error

func (o optionFunc) apply(r *runState) error {
	return o(r)
}

// WithLogger applies a logger that is used for all logging. A discard
// based one is used if none is supplied.
func WithLogger(l logr.Logger) Option {
	return optionFunc(func(r *runState) error {
		r.logger = l
		return nil
	})
}

// WithPolicy applies an OPA policy used against incoming RPC requests.
func WithPolicy(policy string) Option {
	return optionFunc(func(r *runState) error {
		r.policy = policy
		return nil
	})
}

// WithClientPolicy appplies an optional OPA policy for determining outbound decisions.
func WithClientPolicy(policy string) Option {
	return optionFunc(func(r *runState) error {
		r.clientPolicy = policy
		return nil
	})
}

// WithTlsConfig applies a supplied tls.Config object to the gRPC server.
func WithTlsConfig(tlsConfig *tls.Config) Option {
	return optionFunc(func(r *runState) error {
		r.tlsConfig = tlsConfig
		return nil
	})
}

// WithCredSource applies a registered credential source with the mtls package.
func WithCredSource(credSource string) Option {
	return optionFunc(func(r *runState) error {
		r.credSource = credSource
		return nil
	})
}

// WithHostport applies the host:port to run the server.
func WithHostPort(hostport string) Option {
	return optionFunc(func(r *runState) error {
		r.hostport = hostport
		return nil
	})
}

// WithJustification applies the justification param.
// Justification if true requires justification to be set in the
// incoming RPC context Metadata (to the key defined in the telemetry package).
func WithJustification(j bool) Option {
	return optionFunc(func(r *runState) error {
		r.justification = j
		return nil
	})
}

// WithJustificationFunc applies a justification function.
// This function will be called if Justication is true and a justification
// entry is found. The supplied function can then do any validation it wants
// in order to ensure it's compliant.
func WithJustificationHook(hook func(string) error) Option {
	return optionFunc(func(r *runState) error {
		r.justificationFunc = hook
		return nil
	})
}

// WithUnaryInterceptor adds an additional unary server interceptor.
// These become any additional interceptors to be added to unary RPCs
// served from this instance. They will be added after logging and authz checks.
func WithUnaryInterceptor(i grpc.UnaryServerInterceptor) Option {
	return optionFunc(func(r *runState) error {
		r.unaryInterceptors = append(r.unaryInterceptors, i)
		return nil
	})
}

// WithUnaryClientInterceptor adds an additional unary client interceptor.
// These become any additional interceptors to be added to outbound unary RPCs
// performed from this instance. They will be added after logging and authz checks.
func WithUnaryClientInterceptor(i grpc.UnaryClientInterceptor) Option {
	return optionFunc(func(r *runState) error {
		r.unaryClientInterceptors = append(r.unaryClientInterceptors, i)
		return nil
	})
}

// WithStreamInterceptor adds an additional stream server interceptor.
// These become any additional interceptors to be added to streaming RPCs
// served from this instance. They will be added after logging and authz checks.
func WithStreamInterceptor(i grpc.StreamServerInterceptor) Option {
	return optionFunc(func(r *runState) error {
		r.streamInterceptors = append(r.streamInterceptors, i)
		return nil
	})
}

// WithStreamClientInterceptor adds an additional stream client interceptor.
// These become any additional interceptors to be added to outbound streaming RPCs
// performed from this instance. They will be added after logging and authz checks.
func WithStreamClientInterceptor(i grpc.StreamClientInterceptor) Option {
	return optionFunc(func(r *runState) error {
		r.streamClientInterceptors = append(r.streamClientInterceptors, i)
		return nil
	})
}

// WithAuthzHook adds an additional authz hook to be applied to the server.
func WithAuthzHook(hook rpcauth.RPCAuthzHook) Option {
	return optionFunc(func(r *runState) error {
		r.authzHooks = append(r.authzHooks, hook)
		return nil
	})
}

// WithRawServerOption allows one access to the RPC Server object. Generally this is done to add additional
// registration functions for RPC services to be done before starting the server.
func WithRawServerOption(s func(*grpc.Server)) Option {
	return optionFunc(func(r *runState) error {
		r.services = append(r.services, s)
		return nil
	})
}

// Run takes the given context and RunState along with any authz hooks and starts up a sansshell proxy server
// using the flags above to provide credentials. An address hook (based on the remote host) with always be added.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, opts ...Option) {
	rs := &runState{
		logger: logr.Discard(), // Set a default so we can use below.
	}
	for _, o := range opts {
		if err := o.apply(rs); err != nil {
			rs.logger.Error(err, "error applying option")
			os.Exit(1)
		}
	}

	serverCreds, err := mtls.LoadServerCredentials(ctx, rs.credSource)
	if err != nil {
		rs.logger.Error(err, "mtls.LoadServerCredentials", "credsource", rs.credSource)
		os.Exit(1)
	}
	clientCreds, err := mtls.LoadClientCredentials(ctx, rs.credSource)
	if err != nil {
		rs.logger.Error(err, "mtls.LoadClientCredentials", "credsource", rs.credSource)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", rs.hostport)
	if err != nil {
		rs.logger.Error(err, "net.Listen", "hostport", rs.hostport)
		os.Exit(1)
	}
	rs.logger.Info("listening", "hostport", rs.hostport)

	addressHook := rpcauth.HookIf(rpcauth.HostNetHook(lis.Addr()), func(input *rpcauth.RPCAuthInput) bool {
		return input.Host == nil || input.Host.Net == nil
	})
	justificationHook := rpcauth.HookIf(rpcauth.JustificationHook(rs.justificationFunc), func(input *rpcauth.RPCAuthInput) bool {
		return rs.justification
	})

	h := []rpcauth.RPCAuthzHook{addressHook, justificationHook}
	h = append(h, rs.authzHooks...)
	authz, err := rpcauth.NewWithPolicy(ctx, rs.policy, h...)
	if err != nil {
		rs.logger.Error(err, "rpcauth.NewWithPolicy")
		os.Exit(1)
	}

	var clientAuthz *rpcauth.Authorizer
	if rs.clientPolicy != "" {
		clientAuthz, err = rpcauth.NewWithPolicy(ctx, rs.clientPolicy)
		if err != nil {
			rs.logger.Error(err, "client rpcauth.NewWithPolicy")
			os.Exit(1)
		}
	}

	// We always have the logger but might need to chain if we're also doing client outbound OPA checks.
	unaryClient := []grpc.UnaryClientInterceptor{
		telemetry.UnaryClientLogInterceptor(rs.logger),
	}
	streamClient := []grpc.StreamClientInterceptor{
		telemetry.StreamClientLogInterceptor(rs.logger),
	}
	if clientAuthz != nil {
		unaryClient = append(unaryClient, clientAuthz.AuthorizeClient)
		streamClient = append(streamClient, clientAuthz.AuthorizeClientStream)
	}
	unaryClient = append(unaryClient, rs.unaryClientInterceptors...)
	streamClient = append(streamClient, rs.streamClientInterceptors...)
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(clientCreds),
		grpc.WithChainUnaryInterceptor(unaryClient...),
		grpc.WithChainStreamInterceptor(streamClient...),
	}
	targetDialer := server.NewDialer(dialOpts...)

	svcMap := server.LoadGlobalServiceMap()
	rs.logger.Info("loaded service map", "serviceMap", svcMap)
	server := server.New(targetDialer, authz)

	// Even though the proxy RPC is streaming we have unary RPCs (logging, reflection) we
	// also need to properly auth and log.
	unaryServer := []grpc.UnaryServerInterceptor{
		telemetry.UnaryServerLogInterceptor(rs.logger),
		authz.Authorize,
	}
	unaryServer = append(unaryServer, rs.unaryInterceptors...)
	streamServer := []grpc.StreamServerInterceptor{
		telemetry.StreamServerLogInterceptor(rs.logger),
		authz.AuthorizeStream,
	}
	streamServer = append(streamServer, rs.streamInterceptors...)
	serverOpts := []grpc.ServerOption{
		grpc.Creds(serverCreds),
		grpc.ChainUnaryInterceptor(unaryServer...),
		grpc.ChainStreamInterceptor(streamServer...),
	}
	g := grpc.NewServer(serverOpts...)

	// We always register the proxy.
	server.Register(g)

	// Now loop over any other registered and call them.
	for _, s := range rs.services {
		s(g)
	}

	rs.logger.Info("initialized proxy service", "credsource", rs.credSource)
	rs.logger.Info("serving..")

	if err := g.Serve(lis); err != nil {
		rs.logger.Error(err, "grpcserver.Serve()")
		os.Exit(1)
	}
}
