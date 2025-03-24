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

package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"github.com/Snowflake-Labs/sansshell/auth/rpcauthz"
	"log"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"go.opentelemetry.io/otel"
	prometheus_exporter "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetricsdk "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc"
	channelz "google.golang.org/grpc/channelz/service"
	"google.golang.org/grpc/reflection"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"github.com/Snowflake-Labs/sansshell/cmd/proxy-server/server"
	"github.com/Snowflake-Labs/sansshell/cmd/util"
	"github.com/Snowflake-Labs/sansshell/services/mpa/mpahooks"
	ss "github.com/Snowflake-Labs/sansshell/services/sansshell/server"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"

	// Import services here to make them proxy-able
	_ "github.com/Snowflake-Labs/sansshell/services/ansible"
	_ "github.com/Snowflake-Labs/sansshell/services/dns"
	_ "github.com/Snowflake-Labs/sansshell/services/exec"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	_ "github.com/Snowflake-Labs/sansshell/services/httpoverrpc"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile"
	_ "github.com/Snowflake-Labs/sansshell/services/mpa"
	_ "github.com/Snowflake-Labs/sansshell/services/network"
	_ "github.com/Snowflake-Labs/sansshell/services/packages"
	_ "github.com/Snowflake-Labs/sansshell/services/process"
	_ "github.com/Snowflake-Labs/sansshell/services/sansshell"
	_ "github.com/Snowflake-Labs/sansshell/services/service"
	_ "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	_ "github.com/Snowflake-Labs/sansshell/services/tlsinfo"

	_ "github.com/Snowflake-Labs/sansshell/services/fdb/server"

	// Import the sansshell server code so we can use Version.
	ssserver "github.com/Snowflake-Labs/sansshell/services/sansshell/server"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string

	policyFlag       = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile       = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
	clientPolicyFlag = flag.String("client-policy", "", "OPA policy for outbound client actions (i.e. connecting to sansshell servers). If empty no policy is applied.")
	clientPolicyFile = flag.String("client-policy-file", "", "Path to a file with a client OPA.  If empty uses --client-policy")
	hostport         = flag.String("hostport", "localhost:50043", "Where to listen for connections.")
	debugport        = flag.String("debugport", "localhost:50045", "A separate port for http debug pages. Set to an empty string to disable.")
	metricsport      = flag.String("metricsport", "localhost:50046", "Http endpoint for exposing metrics")
	credSource       = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS creds (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	verbosity        = flag.Int("v", 0, "Verbosity level. > 0 indicates more extensive logging")
	validate         = flag.Bool("validate", false, "If true will evaluate the policy and then exit (non-zero on error)")
	justification    = flag.Bool("justification", false, "If true then justification (which is logged and possibly validated) must be passed along in the client context Metadata with the key '"+rpcauthz.ReqJustKey+"'")
	version          bool
)

func init() {
	flag.StringVar(&mtlsFlags.ClientCertFile, "client-cert", mtlsFlags.ClientCertFile, "Path to this client's x509 cert, PEM format")
	flag.StringVar(&mtlsFlags.ClientKeyFile, "client-key", mtlsFlags.ClientKeyFile, "Path to this client's key")
	flag.StringVar(&mtlsFlags.ServerCertFile, "server-cert", mtlsFlags.ServerCertFile, "Path to an x509 server cert, PEM format")
	flag.StringVar(&mtlsFlags.ServerKeyFile, "server-key", mtlsFlags.ServerKeyFile, "Path to the server's TLS key")
	flag.StringVar(&mtlsFlags.RootCAFile, "root-ca", mtlsFlags.RootCAFile, "The root of trust for remote identities, PEM format")

	flag.BoolVar(&version, "version", false, "Returns the server built version from the sansshell server package")
}

func main() {
	flag.Parse()

	if version {
		fmt.Printf("Version: %s\n", ssserver.Version)
		os.Exit(0)
	}

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-proxy")
	stdr.SetVerbosity(*verbosity)

	// Setup exporter using the default prometheus registry
	exporter, err := prometheus_exporter.New()
	if err != nil {
		log.Fatalf("failed to create prometheus exporter: %v\n", err)
	}
	otel.SetMeterProvider(otelmetricsdk.NewMeterProvider(
		otelmetricsdk.WithReader(exporter),
	))
	meter := otel.Meter("sansshell-proxy")
	recorder, err := metrics.NewOtelRecorder(meter, metrics.WithMetricNamePrefix("sansshell-proxy"))
	if err != nil {
		log.Fatalf("failed to create OtelRecorder: %v\n", err)
	}

	policy := util.ChoosePolicy(logger, defaultPolicy, *policyFlag, *policyFile)
	clientPolicy := util.ChoosePolicy(logger, "", *clientPolicyFlag, *clientPolicyFile)
	ctx := logr.NewContext(context.Background(), logger)
	ctx = metrics.NewContextWithRecorder(ctx, recorder)

	parsed, err := opa.NewAuthzPolicy(ctx, policy, opa.WithDenialHintsQuery("data.sansshell.authz.denial_hints"))
	if err != nil {
		log.Fatalf("Invalid policy: %v\n", err)
	}
	if *validate {
		fmt.Println("Policy passes.")
		os.Exit(0)
	}

	// Create a an instance of logging/version for the proxy server itself.
	srv := &ss.Server{}

	server.Run(ctx,
		server.WithLogger(logger),
		server.WithParsedPolicy(parsed),
		server.WithClientPolicy(clientPolicy),
		server.WithCredSource(*credSource),
		server.WithHostPort(*hostport),
		server.WithJustification(*justification),
		server.WithAuthzHook(rpcauthz.PeerPrincipalFromCertHook()),
		server.WithAuthzHook(mpahooks.ProxyMPAAuthzHook()),
		server.WithRawServerOption(func(s *grpc.Server) { reflection.Register(s) }),
		server.WithRawServerOption(func(s *grpc.Server) { channelz.RegisterChannelzServiceToServer(s) }),
		server.WithRawServerOption(srv.Register),
		server.WithDebugPort(*debugport),
		server.WithMetricsPort(*metricsport),
		server.WithMetricsRecorder(recorder),
	)
}
