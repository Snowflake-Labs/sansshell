package healthcheck

// To regenerate the proto headers if the proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative healthcheck.proto

import (
	"context"
	"log"

	"github.com/snowflakedb/unshelled/services"
	grpc "google.golang.org/grpc"
)

// server is used to implement the gRPC server
type server struct {
	UnimplementedHealthCheckServer
}

// Ok always returns an Empty proto without error
func (s *server) Ok(ctx context.Context, in *Empty) (*Empty, error) {
	log.Printf("Received HealthCheck request")
	return &Empty{}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterHealthCheckServer(gs, s)
}

func init() {
	services.RegisterUnshelledService(&server{})
}
