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

// Serve wraps up BuildServer in a succinct API for callers
func Serve(hostport string, c credentials.TransportCredentials, policy string, logger logr.Logger) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	mu.Lock()
	srv, err = BuildServer(c, policy, lis.Addr(), logger)
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

// BuildServer creates a gRPC server, attaches the OPA policy interceptor,
// registers all of the imported SansShell modules. Separating this from Serve
// primarily facilitates testing.
func BuildServer(c credentials.TransportCredentials, policy string, address net.Addr, logger logr.Logger) (*grpc.Server, error) {
	authz, err := rpcauth.NewWithPolicy(context.Background(), policy, rpcauth.HostNetHook(address))
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
