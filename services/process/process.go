package process

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative process.proto

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/Snowflake-Labs/sansshell/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server is used to implement the gRPC server
type server struct {
}

func (s *server) List(ctx context.Context, req *ListRequest) (*ListReply, error) {
	log.Printf("Received request for List: %+v", req)

	cmdName := *psBin
	options, err := psOptions()
	if err != nil {
		return nil, status.Error(codes.Unimplemented, fmt.Sprintf("can't determine ps options: %v", err))
	}

	// We gather all the processes up and then filter by pid if needed at the end.
	cmd := exec.CommandContext(ctx, cmdName, options...)
	var stderrBuf, stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := cmd.Wait(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("command exited with error: %v\n%s", err, string(stderrBuf.Bytes())))
	}

	errBuf := stderrBuf.Bytes()
	if len(errBuf) != 0 {
		return nil, status.Error(codes.Internal, fmt.Sprintf("unexpected error output:\n%s", string(errBuf)))
	}

	entries, err := parser(&stdoutBuf)

	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("unexpected parsing error: %v", err))
	}

	reply := &ListReply{}
	if len(req.Pids) != 0 {
		for _, pid := range req.Pids {
			if _, ok := entries[pid]; !ok {
				return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("pid %d does not exist", pid))
			}

			reply.ProcessEntries = append(reply.ProcessEntries, entries[pid])
		}
		return reply, nil
	}

	// If not filtering fill everything in and return. We don't guarentee any ordering.
	for _, e := range entries {
		reply.ProcessEntries = append(reply.ProcessEntries, e)
	}
	return reply, nil
}

func (s *server) GetStacks(ctx context.Context, req *GetStacksRequest) (*GetStacksReply, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *server) GetJavaStacks(ctx context.Context, req *GetJavaStacksRequest) (*GetJavaStacksReply, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *server) GetCore(req *GetCoreRequest, stream Process_GetCoreServer) error {
	return status.Error(codes.Unimplemented, "")
}

func (s *server) GetJavaHeapDump(req *GetJavaHeapDumpRequest, stream Process_GetJavaHeapDumpServer) error {
	return status.Error(codes.Unimplemented, "")
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterProcessServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
