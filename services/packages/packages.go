package packages

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative packages.proto

import (
	"context"
	"errors"

	"github.com/Snowflake-Labs/sansshell/services"
	grpc "google.golang.org/grpc"
)

// server is used to implement the gRPC server
type server struct{}

func (s *server) Install(ctx context.Context, req *InstallRequest) (*InstallReply, error) {
	return nil, errors.New("not implemented")
}

func (s *server) Update(ctx context.Context, req *UpdateRequest) (*UpdateReply, error) {
	return nil, errors.New("not implemented")
}

func (s *server) ListInstalled(ctx context.Context, req *ListInstalledRequest) (*ListInstalledReply, error) {
	return nil, errors.New("not implemented")
}

func (s *server) RepoList(ctx context.Context, req *RepoListRequest) (*RepoListReply, error) {
	return nil, errors.New("not implemented")
}

// Install is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterPackagesServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
