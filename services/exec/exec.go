package exec

// To regenerate the proto headers if the .proto changes, just run go generate
// This comment encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative exec.proto

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/Snowflake-Labs/sansshell/services"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server is used to implement the gRPC server
type server struct{}

// Run executes command and returns result
func (s *server) Run(ctx context.Context, req *ExecRequest) (res *ExecResponse, err error) {
	cmdName := req.Command
	cmdArgs := req.Args

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	var errBuf, outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Errorf(codes.Internal, "can't start program: %v", err)
	}

	err = cmd.Wait()
	if err != nil {
		return &ExecResponse{
			Stdout:  outBuf.Bytes(),
			Stderr:  errBuf.Bytes(),
			RetCode: int32(cmd.ProcessState.ExitCode()),
		}, nil
	}

	return &ExecResponse{Stderr: errBuf.Bytes(), Stdout: outBuf.Bytes(), RetCode: 0}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterExecServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
