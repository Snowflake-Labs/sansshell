package localfile

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative localfile.proto

import (
	"context"
	"io"
	"log"
	"math"
	"os"

	"github.com/Snowflake-Labs/sansshell/services"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server is used to implement the gRPC server
type server struct {
}

// TODO(jchacon): Make this configurable
// The chunk size we use when sending replies to Read on
// the stream.
var chunkSize = 128 * 1024

// Read returns the contents of the named file
func (s *server) Read(in *ReadRequest, stream LocalFile_ReadServer) error {
	file := in.GetFilename()
	log.Printf("Received request for: %v", file)
	f, err := os.Open(file)
	if err != nil {
		log.Printf("Can't open file %s: %v", file, err)
		return err
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Can't close file %s: %v", file, err)
		}
	}()

	// Seek forward if requested
	if s := in.GetOffset(); s != 0 {
		whence := 0
		// If negative we're tailing from the end so
		// negate the sign and set whence.
		if s < 0 {
			whence = 2
		}
		if _, err := f.Seek(s, whence); err != nil {
			return err
		}
	}

	max := in.GetLength()
	if max == 0 {
		max = math.MaxInt64
	}

	buf := make([]byte, chunkSize)

	reader := io.LimitReader(f, max)

	for {
		n, err := reader.Read(buf)

		// If we got EOF we're done.
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		// Only send over the number of bytes we actually read or
		// else we'll send over garbage in the last packet potentially.
		if err := stream.Send(&ReadReply{Contents: buf[:n]}); err != nil {
			return err
		}

		// If we got back less than a full chunk we're done.
		if n < chunkSize {
			break
		}
	}
	return nil
}

func (s *server) Stat(ctx context.Context, in *StatRequest) (*StatReply, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *server) Sum(ctx context.Context, in *SumRequest) (*SumReply, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterLocalFileServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
