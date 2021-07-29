package exec

import (
	"context"
	"io/ioutil"
	"os/exec"

	"github.com/snowflakedb/unshelled/services"
	grpc "google.golang.org/grpc"
)

// server is used to implement the gRPC server
type server struct{}

// Exec executes command and returns result
func (s *server) Exec(ctx context.Context, req *ExecRequest) (res *ExecResponse, err error) {
	cmdName := req.Command[0]
	cmdArgs := req.Command[1:]

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

	errbuf, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, err
	}
	outbuf, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}

	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	return &ExecResponse{Error: errbuf, Output: outbuf}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterExecServer(gs, s)
}

func init() {
	services.RegisterUnshelledService(&server{})
}