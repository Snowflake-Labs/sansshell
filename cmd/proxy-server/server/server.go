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

// Package server provides functioanlity so that other uses of sansshell can provide their
// own main.go without having to cargo-cult everything across for common use cases. i.e. simply
// adding your own authz hooks but using the standard modules. Or adding additional modules that
// are locally defined.
package server

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/server"
	"github.com/Snowflake-Labs/sansshell/telemetry"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"

	// Import services here to make them proxy-able
	_ "github.com/Snowflake-Labs/sansshell/services/ansible"
	_ "github.com/Snowflake-Labs/sansshell/services/exec"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile"
	_ "github.com/Snowflake-Labs/sansshell/services/packages"
	_ "github.com/Snowflake-Labs/sansshell/services/process"
	_ "github.com/Snowflake-Labs/sansshell/services/service"
)

var (
	hostport   = flag.String("hostport", "localhost:50043", "Where to listen for connections.")
	credSource = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS creds (one of [%s])", strings.Join(mtls.Loaders(), ",")))
)

// Run takes the given context, logger, policy and any authz hooks and starts up a sansshell proxy server
// using the flags above to provide credentials. An address hook (based on the remote host) with always be added.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, logger logr.Logger, policy string, hooks ...rpcauth.RPCAuthzHook) {
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

	addressHook := rpcauth.HookIf(rpcauth.HostNetHook(lis.Addr()), func(input *rpcauth.RPCAuthInput) bool {
		return input.Host == nil || input.Host.Net == nil
	})
	h := []rpcauth.RPCAuthzHook{addressHook}
	h = append(h, hooks...)
	authz, err := rpcauth.NewWithPolicy(ctx, policy, h...)
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
