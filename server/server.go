package server

import (
  "context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/Snowflake-Labs/sansshell/services"
)

// Serve wraps up BuildServer in a succinct API for callers
func Serve(hostport string, c credentials.TransportCredentials, policy string) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s, err := BuildServer(c, policy)
	if err != nil {
		return err
	}

	return s.Serve(lis)
}

// BuildServer creates a gRPC server, attaches the OPA policy interceptor,
// registers all of the imported SansShell modules. Separating this from Serve
// primarily facilitates testing.
func BuildServer(c credentials.TransportCredentials, policy string) (*grpc.Server, error) {
	o, err := NewOPA(context.Background(), policy)
	if err != nil {
		return &grpc.Server{}, fmt.Errorf("NewOpa: %w", err)
	}
	s := grpc.NewServer(grpc.Creds(c), grpc.UnaryInterceptor(o.Authorize), grpc.StreamInterceptor(o.AuthorizeStream))
	for _, sansShellService := range services.ListServices() {
		sansShellService.Register(s)
	}
	return s, nil
}
