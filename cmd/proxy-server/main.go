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

	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/server"

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

	ctx := context.Background()

	serverCreds, err := mtls.LoadServerCredentials(ctx, *credSource)
	if err != nil {
		log.Fatalf("mtls.LoadServerCredentials(%s) %v", *credSource, err)
	}
	clientCreds, err := mtls.LoadClientCredentials(ctx, *credSource)
	if err != nil {
		log.Fatalf("mtls.LoadClientCredentials(%s) %v", *credSource, err)
	}

	lis, err := net.Listen("tcp", *hostport)
	if err != nil {
		log.Fatalf("net.Listen(%s): %v", *hostport, err)
	}
	log.Println("listening on", *hostport)

	// TODO(jallie): implement the ability to 'hot reload' policy, since
	// that could likely be done underneath the authorizer, with little
	// disruption to existing connections.
	policy := defaultPolicy
	if *policyFile != "" {
		data, err := os.ReadFile(*policyFile)
		if err != nil {
			log.Fatalf("error reading policy file %s : %v", *policyFile, err)
		}
		policy = string(data)
		log.Println("using authorization policy from ", *policyFile)
	} else {
		log.Println("using default authorization policy")
	}

	addressHook := rpcauth.HookIf(rpcauth.HostNetHook(lis.Addr()), func(input *rpcauth.RpcAuthInput) bool {
		return input.Host == nil || input.Host.Net == nil
	})
	authz, err := rpcauth.NewWithPolicy(ctx, policy, addressHook)
	if err != nil {
		log.Fatalf("error initializing authorization : %v", err)
	}

	targetDialer := server.NewDialer(grpc.WithTransportCredentials(clientCreds))
	server := server.New(targetDialer, authz)

	g := grpc.NewServer(grpc.Creds(serverCreds), grpc.StreamInterceptor(authz.AuthorizeStream))

	server.Register(g)
	log.Println("initialized Proxy service using credentials from", *credSource)

	if err := g.Serve(lis); err != nil {
		log.Fatalf("gRPCServer.Serve() %v", err)
	}
}
