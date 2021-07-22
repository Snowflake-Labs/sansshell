package server

import (
	"fmt"
	"net"

	"google.golang.org/grpc"

	"github.com/snowflakedb/unshelled/services"
)

// Serve wraps up BuildServer in a succinct API for callers
func Serve(hostport string, policy string) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s, err := BuildServer(lis, policy)
	if err != nil {
		return err
	}

	return s.Serve(lis)
}

// BuildServer creates a gRPC server, attaches the OPA policy interceptor,
// registers all of the imported Unshelled modules. Separating this from Serve
// primarily facilitates testing.
func BuildServer(lis net.Listener, policy string) (*grpc.Server, error) {
	o, err := NewOPA(policy)
	if err != nil {
		return &grpc.Server{}, fmt.Errorf("NewOpa: %w", err)
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(o.Authorize))
	for _, unshelledService := range services.ListServices() {
		unshelledService.Register(s)
	}
	return s, nil
}
