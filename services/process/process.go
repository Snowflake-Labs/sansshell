package process

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative process.proto

import (
	"context"
	"errors"

	"github.com/Snowflake-Labs/sansshell/services"
	grpc "google.golang.org/grpc"
)

// server is used to implement the gRPC server
type server struct {
}

func (s *server) List(ctx context.Context, req *ListRequest) (*ListReply, error) {
	return nil, errors.New("not implemented")
}

func (s *server) GetStacks(ctx context.Context, req *GetStacksRequest) (*GetStacksReply, error) {
	return nil, errors.New("not implemented")
}

func (s *server) GetJavaStacks(ctx context.Context, req *GetJavaStacksRequest) (*GetJavaStacksReply, error) {
	return nil, errors.New("not implemented")
}

func (s *server) GetCore(req *GetCoreRequest, stream Process_GetCoreServer) error {
	return errors.New("not implemented")
}

func (s *server) GetJavaHeapDump(req *GetJavaHeapDumpRequest, stream Process_GetJavaHeapDumpServer) error {
	return errors.New("not implemented")
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterProcessServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
