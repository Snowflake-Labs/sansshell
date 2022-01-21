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
	"log"
	"net"
	"os"
	"strings"

	"github.com/go-logr/stdr"
	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/server"
	"github.com/Snowflake-Labs/sansshell/telemetry"

	// Import services here to make them proxy-able
	_ "github.com/Snowflake-Labs/sansshell/services/ansible"
	_ "github.com/Snowflake-Labs/sansshell/services/exec"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile"
	_ "github.com/Snowflake-Labs/sansshell/services/packages"
	_ "github.com/Snowflake-Labs/sansshell/services/process"
)

//go:embed default-policy.rego
var defaultPolicy string

func main() {
	hostport := flag.String("hostport", "localhost:50043", "Where to listen for connections.")
	credSource := flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS creds (one of [%s])", strings.Join(mtls.Loaders(), ",")))
	policyFile := flag.String("policy-file", "", "Path to an authorization file")

	flag.Parse()

	logOpts := log.Ldate | log.Ltime | log.Lshortfile
	logger := stdr.New(log.New(os.Stderr, "", logOpts)).WithName("sanshell-proxy")

	ctx := context.Background()

	serverCreds, err := mtls.LoadServerCredentials(ctx, *credSource)
	if err != nil {
		logger.Error(err, "mtls.LoadServerCredentials", "credsource", *credSource)
		os.Exit(1)
	}
	clientCreds, err := mtls.LoadClientCredentials(ctx, *credSource)
	if err != nil {
		logger.Error(err, "mtls.LoadClientCredentials", "credsource", *credSource)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", *hostport)
	if err != nil {
		logger.Error(err, "net.Listen", "hostport", *hostport)
		os.Exit(1)
	}
	logger.Info("listening", "hostport", *hostport)

	// TODO(jallie): implement the ability to 'hot reload' policy, since
	// that could likely be done underneath the authorizer, with little
	// disruption to existing connections.
	policy := defaultPolicy
	if *policyFile != "" {
		data, err := os.ReadFile(*policyFile)
		if err != nil {
			logger.Error(err, "policy file read", "file", *policyFile)
			os.Exit(1)
		}
		policy = string(data)
		logger.Info("using authorization policy from file", "file", *policyFile)
	} else {
		logger.Info("using default authorization policy")
	}

	addressHook := rpcauth.HookIf(rpcauth.HostNetHook(lis.Addr()), func(input *rpcauth.RpcAuthInput) bool {
		return input.Host == nil || input.Host.Net == nil
	})
	authz, err := rpcauth.NewWithPolicy(ctx, policy, addressHook)
	if err != nil {
		logger.Error(err, "rpcauth.NewWithPolicy")
		os.Exit(1)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(clientCreds),
		grpc.WithStreamInterceptor(telemetry.StreamClientLogInterceptor(logger)),
	}
	targetDialer := server.NewDialer(dialOpts...)

	svcMap := server.LoadGlobalServiceMap()
	logger.Info("loaded service map", "serviceMap", svcMap)
	server := server.New(targetDialer, authz)

	serverOpts := []grpc.ServerOption{
		grpc.Creds(serverCreds),
		grpc.ChainStreamInterceptor(telemetry.StreamServerLogInterceptor(logger), authz.AuthorizeStream),
	}
	g := grpc.NewServer(serverOpts...)

	server.Register(g)
	logger.Info("initialized proxy service", "credsource", *credSource)
	logger.Info("serving..")

	if err := g.Serve(lis); err != nil {
		logger.Error(err, "grpcserver.Serve()")
		os.Exit(1)
	}
}
