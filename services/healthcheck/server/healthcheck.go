package server

import (
	"context"
	"log"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"google.golang.org/grpc"
)

// server is used to implement the gRPC server
type server struct{}

// Ok always returns an Empty proto without error
func (s *server) Ok(ctx context.Context, in *pb.Empty) (*pb.Empty, error) {
	log.Print("Received request for HealthCheck.Ok")
	return &pb.Empty{}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterHealthCheckServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
