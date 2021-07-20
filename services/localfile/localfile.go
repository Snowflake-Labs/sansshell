package localfile

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative localfile.proto

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/snowflakedb/unshelled/services"
	grpc "google.golang.org/grpc"
)

// server is used to implement the gRPC server
type server struct {
	UnimplementedLocalFileServer
}

// Read returns the contents of the named file
func (s *server) Read(ctx context.Context, in *ReadRequest) (*ReadReply, error) {
	log.Printf("Received request for: %v", in.GetFilename())
	contents, err := ioutil.ReadFile(in.GetFilename())
	if err != nil {
		return nil, err
	}
	return &ReadReply{Contents: contents}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterLocalFileServer(gs, s)
}

func init() {
	services.RegisterUnshelledService(&server{})
}
