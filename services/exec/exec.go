package exec

// To regenerate the proto headers if the .proto changes, just run go generate
// This comment encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative exec.proto

import (
	"context"
	"io/ioutil"
	"os/exec"
	"syscall"

	"github.com/snowflakedb/unshelled/services"
	grpc "google.golang.org/grpc"
)

// server is used to implement the gRPC server
type server struct{}

// Run executes command and returns result
func (s *server) Run(ctx context.Context, req *ExecRequest) (res *ExecResponse, err error) {
	cmdName := req.Command
	cmdArgs := req.Args

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	errBuf, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, err
	}
	outBuf, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}

	err = cmd.Wait()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return nil, err
		}
		pStateSys := exitErr.Sys()
		wStatus, ok := pStateSys.(syscall.WaitStatus)
		if !ok {
			return nil, err
		}
		exitStatus := wStatus.ExitStatus()
		return &ExecResponse{
			Stdout:  outBuf,
			Stderr:  errBuf,
			RetCode: int32(exitStatus),
		}, nil
	}

	return &ExecResponse{Stderr: errBuf, Stdout: outBuf, RetCode: 0}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterExecServer(gs, s)
}

func init() {
	services.RegisterUnshelledService(&server{})
}
