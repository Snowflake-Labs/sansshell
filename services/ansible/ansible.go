package ansible

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative ansible.proto

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os/exec"

	"github.com/Snowflake-Labs/sansshell/services"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ansiblePlaybookBin = flag.String("ansible_playbook_bin", "/usr/bin/ansible-playbook", "Path to ansible-playbook binary")

// A test hook so we can take the args passed and transform them as needed.
var cmdArgsTransform = func(input []string) []string {
	return input
}

// server is used to implement the gRPC server
type server struct{}

func (s *server) Run(ctx context.Context, req *RunRequest) (*RunReply, error) {
	// Basic sanity checking up front.
	if len(req.Playbook) == 0 || req.Playbook[0] != '/' {
		return nil, status.Error(codes.Internal, "playbook path must be a full qualified path")
	}

	cmdArgs := []string{
		"-i",
		"localhost,",         // Keeps it only to this host
		"--connection=local", // Make sure it doesn't try and ssh out
	}

	for _, v := range req.Vars {
		cmdArgs = append(cmdArgs, "-e")
		cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%s", v.Key, v.Value))
	}

	if req.User != "" {
		cmdArgs = append(cmdArgs, "--become")
		cmdArgs = append(cmdArgs, req.User)
	}

	if req.Check {
		cmdArgs = append(cmdArgs, "--check")
	}

	if req.Diff {
		cmdArgs = append(cmdArgs, "--diff")
	}

	if req.Verbose {
		cmdArgs = append(cmdArgs, "-vvv")
	}

	cmdArgs = append(cmdArgs, req.Playbook)

	cmdArgs = cmdArgsTransform(cmdArgs)

	cmd := exec.CommandContext(ctx, *ansiblePlaybookBin, cmdArgs...)
	var stderrBuf, stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	log.Printf("Executing: %s %v", cmd.Path, cmd.Args)
	if err := cmd.Start(); err != nil {
		return nil, status.Errorf(codes.Internal, "can't start ansible-playbook: %v", err)
	}

	cmd.Wait()

	return &RunReply{
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		ReturnCode: int32(cmd.ProcessState.ExitCode()),
	}, nil
}

// Install is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterPlaybookServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
