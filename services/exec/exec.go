package exec

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

	errBuf, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, err
	}
	outBuf, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}

	err = cmd.Wait()
	exitErr, ok := err.(*exec.ExitError)
	if err != nil && ok {
		pStateSys := exitErr.Sys()
		wStatus := pStateSys.(syscall.WaitStatus)
		exitStatus := wStatus.ExitStatus()
		return &ExecResponse{
			Stdout:        outBuf,
			Stderr:        errBuf,
			RetCode:       int32(exitStatus),
		}, nil

	} else if err != nil {
		return nil, err
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

//func byteToString(input []byte) string {
//	return string(input)
//}
//
//func stringToByte(input string) []byte {
//	return []byte(input)
//}

//https://stackoverflow.com/questions/11886531/terminating-a-process-started-with-os-exec-in-golang