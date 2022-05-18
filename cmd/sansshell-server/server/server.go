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
// own main.go without having to cargo-cult everything across for common use cases.
// i.e. adding additional modules that are locally defined.
package server

import (
	"context"
	"fmt"
	"os"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/server"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

// runState encapsulates all of the variable state needed
// to run a proxy server. Documentation provided below in each
// WithXXX function.
type runState struct {
	logger             logr.Logger
	credSource         string
	hostport           string
	policy             string
	justification      bool
	justificationFunc  func(string) error
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	authzHooks         []rpcauth.RPCAuthzHook
}

type Option interface {
	apply(*runState) error
}

type optionFunc func(*runState) error

func (o optionFunc) apply(r *runState) error {
	return o(r)
}

// WithLogger applies a logger that is used for all logging.
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

// WithStreamInterceptor adds an additional stream server interceptor.
// These become any additional interceptors to be added to streaming RPCs
// served from this instance. They will be added after logging and authz checks.
func WithStreamInterceptor(i grpc.StreamServerInterceptor) Option {
	return optionFunc(func(r *runState) error {
		r.streamInterceptors = append(r.streamInterceptors, i)
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

// Run takes the given context and RunState and starts up a sansshell server.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, opts ...Option) {
	rs := &runState{
		logger: logr.Discard(), // Set a default so we can use below.
	}
	for _, o := range opts {
		if err := o.apply(rs); err != nil {
			fmt.Fprintf(os.Stderr, "error applying option: %v\n", err)
			os.Exit(1)
		}
	}

	creds, err := mtls.LoadServerCredentials(ctx, rs.credSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading server creds: %v\n", err)
		rs.logger.Error(err, "mtls.LoadServerCredentials", "credsource", rs.credSource)
		os.Exit(1)
	}

	justificationHook := rpcauth.HookIf(rpcauth.JustificationHook(rs.justificationFunc), func(input *rpcauth.RPCAuthInput) bool {
		return rs.justification
	})

	var serverOpts []server.Option
	serverOpts = append(serverOpts, server.WithCredentials(creds))
	serverOpts = append(serverOpts, server.WithPolicy(rs.policy))
	serverOpts = append(serverOpts, server.WithLogger(rs.logger))
	serverOpts = append(serverOpts, server.WithAuthzHook(justificationHook))
	for _, a := range rs.authzHooks {
		serverOpts = append(serverOpts, server.WithAuthzHook(a))
	}
	for _, u := range rs.unaryInterceptors {
		serverOpts = append(serverOpts, server.WithUnaryInterceptor(u))
	}
	for _, s := range rs.streamInterceptors {
		serverOpts = append(serverOpts, server.WithStreamInterceptor(s))
	}
	if err := server.Serve(rs.hostport, serverOpts...); err != nil {
		fmt.Fprintf(os.Stderr, "Can't serve: %v\n", err)
		rs.logger.Error(err, "server.Serve", "hostport", rs.hostport)
		os.Exit(1)
	}
}
