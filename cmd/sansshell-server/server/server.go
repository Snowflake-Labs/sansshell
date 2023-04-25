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
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/go-logr/logr"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/server"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"google.golang.org/grpc/credentials"
)

// runState encapsulates all of the variable state needed
// to run a proxy server. Documentation provided below in each
// WithXXX function.
type runState struct {
	logger             logr.Logger
	credSource         string
	tlsConfig          *tls.Config
	hostport           string
	debugport          string
	debughandler       *http.ServeMux
	metricsport        string
	metricshandler     *http.ServeMux
	metricsRecorder    metrics.MetricsRecorder
	policy             *opa.AuthzPolicy
	justification      bool
	justificationFunc  func(string) error
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	authzHooks         []rpcauth.RPCAuthzHook
	services           []func(*grpc.Server)
}

type Option interface {
	apply(context.Context, *runState) error
}

type optionFunc func(context.Context, *runState) error

func (o optionFunc) apply(ctx context.Context, r *runState) error {
	return o(ctx, r)
}

// WithLogger applies a logger that is used for all logging. A discard
// based one is used if none is supplied.
func WithLogger(l logr.Logger) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.logger = l
		return nil
	})
}

// WithPolicy applies an OPA policy used against incoming RPC requests.
func WithPolicy(policy string) Option {
	return optionFunc(func(ctx context.Context, r *runState) error {
		p, err := opa.NewAuthzPolicy(ctx, policy)
		if err != nil {
			return err
		}
		r.policy = p
		return nil
	})
}

// WithParsedPolicy applies an already-parsed OPA policy used against incoming RPC requests.
func WithParsedPolicy(policy *opa.AuthzPolicy) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.policy = policy
		return nil
	})
}

// WithTlsConfig applies a supplied tls.Config object to the gRPC server.
func WithTlsConfig(tlsConfig *tls.Config) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.tlsConfig = tlsConfig
		return nil
	})
}

// WithCredSource applies a registered credential source with the mtls package.
func WithCredSource(credSource string) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.credSource = credSource
		return nil
	})
}

// WithHostport applies the host:port to run the server.
func WithHostPort(hostport string) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.hostport = hostport
		return nil
	})
}

// WithJustification applies the justification param.
// Justification if true requires justification to be set in the
// incoming RPC context Metadata (to the key defined in the telemetry package).
func WithJustification(j bool) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.justification = j
		return nil
	})
}

// WithJustificationFunc applies a justification function.
// This function will be called if Justication is true and a justification
// entry is found. The supplied function can then do any validation it wants
// in order to ensure it's compliant.
func WithJustificationHook(hook func(string) error) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.justificationFunc = hook
		return nil
	})
}

// WithUnaryInterceptor adds an additional unary server interceptor.
// These become any additional interceptors to be added to unary RPCs
// served from this instance. They will be added after logging and authz checks.
func WithUnaryInterceptor(i grpc.UnaryServerInterceptor) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.unaryInterceptors = append(r.unaryInterceptors, i)
		return nil
	})
}

// WithStreamInterceptor adds an additional stream server interceptor.
// These become any additional interceptors to be added to streaming RPCs
// served from this instance. They will be added after logging and authz checks.
func WithStreamInterceptor(i grpc.StreamServerInterceptor) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.streamInterceptors = append(r.streamInterceptors, i)
		return nil
	})
}

// WithAuthzHook adds an additional authz hook to be applied to the server.
func WithAuthzHook(hook rpcauth.RPCAuthzHook) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.authzHooks = append(r.authzHooks, hook)
		return nil
	})
}

// WithRawServerOption allows one access to the RPC Server object. Generally this is done to add additional
// registration functions for RPC services to be done before starting the server.
func WithRawServerOption(s func(*grpc.Server)) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		r.services = append(r.services, s)
		return nil
	})
}

// WithDebugPort opens an additional port for a http debug page.
//
// This is meant for humans. The format of the debug pages may change over time.
func WithDebugPort(addr string) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				fmt.Fprintln(w, `<!DOCTYPE html>
			Hi, I'm a sansshell server. Maybe you want one of the following pages.
			<ul>
			<li><a href="/debug/pprof">/debug/pprof</a></li>
			<li><a href="/metrics">/metrics</a></li>
			</ul>`)
			} else {
				http.NotFound(w, r)
				return
			}
		})
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		r.debugport = addr
		r.debughandler = mux
		return nil
	})
}

// WithMetricsRecorder enables metric instrumentations by inserting grpc metric interceptors
// and attaching recorder to the server runstate
func WithMetricsRecorder(recorder metrics.MetricsRecorder) Option {
	return optionFunc(func(ctx context.Context, r *runState) error {
		r.metricsRecorder = recorder
		// Instrument gRPC Server
		grpcServerMetrics := grpc_prometheus.NewServerMetrics(
			grpc_prometheus.WithServerHandlingTimeHistogram(),
		)
		errRegister := prometheus.Register(grpcServerMetrics)
		if errRegister != nil {
			return fmt.Errorf("fail to register grpc server metrics: %s", errRegister)
		}
		r.unaryInterceptors = append(r.unaryInterceptors,
			metrics.UnaryServerMetricsInterceptor(recorder),
			grpcServerMetrics.UnaryServerInterceptor(),
		)
		r.streamInterceptors = append(r.streamInterceptors,
			metrics.StreamServerMetricsInterceptor(recorder),
			grpcServerMetrics.StreamServerInterceptor(),
		)
		return nil
	})
}

// WithMetricsPort opens a HTTP endpoint for publishing metrics at the given addr
// and initializes metrics exporter.
// This endpoint is to be scraped by a Prometheus-style metrics scraper.
// It can be accessed at http://{addr}/metrics
func WithMetricsPort(addr string) Option {
	return optionFunc(func(ctx context.Context, r *runState) error {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		r.metricsport = addr
		r.metricshandler = mux

		return nil
	})
}

// WithOtelTracing adds the OpenTelemetry gRPC interceptors to both stream and unary servers
// The interceptors collect and export tracing data for gRPC requests and responses
func WithOtelTracing(interceptorOpts ...otelgrpc.Option) Option {
	return optionFunc(func(_ context.Context, r *runState) error {
		interceptorOpts = append(interceptorOpts,
			otelgrpc.WithMeterProvider(otelmetric.NewNoopMeterProvider()), // We don't want otel grpc metrics so discard them
		)
		r.unaryInterceptors = append(r.unaryInterceptors,
			otelgrpc.UnaryServerInterceptor(interceptorOpts...),
		)
		r.streamInterceptors = append(r.streamInterceptors,
			otelgrpc.StreamServerInterceptor(interceptorOpts...),
		)
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
		if err := o.apply(ctx, rs); err != nil {
			rs.logger.Error(err, "error applying option")
			os.Exit(1)
		}
	}

	// If there's a debug port, we want to start it early
	if rs.debughandler != nil && rs.debugport != "" {
		go func() {
			rs.logger.Error(http.ListenAndServe(rs.debugport, rs.debughandler), "Debug handler unexpectedly exited")
		}()
	}

	// Start metrics endpoint if both metrics port and handler are configured
	if rs.metricshandler != nil && rs.metricsport != "" {
		go func() {
			rs.logger.Error(http.ListenAndServe(rs.metricsport, rs.metricshandler), "Metrics handler unexpectedly exited")
		}()
	}

	creds, err := extractTransportCredentialsFromRunState(ctx, rs)
	if err != nil {
		rs.logger.Error(err, "unable to extract transport credentials from runstate", "credsource", rs.credSource)
		os.Exit(1)
	}

	justificationHook := rpcauth.HookIf(rpcauth.JustificationHook(rs.justificationFunc), func(input *rpcauth.RPCAuthInput) bool {
		return rs.justification
	})

	var serverOpts []server.Option
	serverOpts = append(serverOpts, server.WithCredentials(creds))
	serverOpts = append(serverOpts, server.WithParsedPolicy(rs.policy))
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
	for _, s := range rs.services {
		serverOpts = append(serverOpts, server.WithRawServerOption(s))
	}
	if err := server.Serve(rs.hostport, serverOpts...); err != nil {
		rs.logger.Error(err, "server.Serve", "hostport", rs.hostport)
		os.Exit(1)
	}
}

// extractTransportCredentialsFromRunState extracts transport credentials from runState. Will error if both credSource and tlsConfig are specified
func extractTransportCredentialsFromRunState(ctx context.Context, rs *runState) (credentials.TransportCredentials, error) {
	var creds credentials.TransportCredentials
	var err error
	if rs.credSource != "" && rs.tlsConfig != nil {
		return nil, fmt.Errorf("both credSource and tlsConfig are defined")
	}
	if rs.credSource != "" {
		creds, err = mtls.LoadServerCredentials(ctx, rs.credSource)
		if err != nil {
			return nil, err
		}
	} else {
		creds = credentials.NewTLS(rs.tlsConfig)
	}
	return creds, nil
}
