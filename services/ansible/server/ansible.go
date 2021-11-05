package server

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/ansible"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"google.golang.org/grpc"
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

var re = regexp.MustCompile("[^a-zA-Z0-9_/]+")

func (s *server) Run(ctx context.Context, req *pb.RunRequest) (*pb.RunReply, error) {
	log.Printf("Received request for Ansible.Run: %+v", req)

	// Basic sanity checking up front.
	if req.Playbook == "" {
		return nil, status.Error(codes.InvalidArgument, "playbook path must be filled in")
	}
	if !filepath.IsAbs(req.Playbook) {
		return nil, status.Errorf(codes.InvalidArgument, "playbook %s must be an absolute path", req.Playbook)
	}
	if req.Playbook != filepath.Clean(req.Playbook) {
		return nil, status.Errorf(codes.InvalidArgument, "playbook %s must be a clean path", req.Playbook)
	}

	// Make sure it's a valid file and nothing something which might be malicious like
	// /some/path && rm -rf /
	stat, err := os.Stat(req.Playbook)
	if err != nil || stat.IsDir() {
		return nil, status.Errorf(codes.InvalidArgument, "%s is not a valid file", req.Playbook)
	}

	cmdArgs := []string{
		"-i",
		"localhost,",         // Keeps it only to this host
		"--connection=local", // Make sure it doesn't try and ssh out
	}

	for _, v := range req.Vars {
		if v.Key != re.ReplaceAllString(v.Key, "") || v.Value != re.ReplaceAllString(v.Value, "") {
			return nil, status.Errorf(codes.InvalidArgument, "vars must contain key/value that is only contains %s - '%s=%s' is invalid", re.String(), v.Key, v.Value)
		}
		cmdArgs = append(cmdArgs, "-e")
		cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%s", v.Key, v.Value))
	}

	if req.User != "" {
		if req.User != re.ReplaceAllString(req.User, "") {
			return nil, status.Errorf(codes.InvalidArgument, "user must only contain %s - %q is invalid", re.String(), req.User)
		}
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

	run, err := util.RunCommand(ctx, *ansiblePlaybookBin, cmdArgs, false)
	if err != nil {
		return nil, err
	}

	return &pb.RunReply{
		Stdout:     run.Stdout.String(),
		Stderr:     run.Stderr.String(),
		ReturnCode: int32(run.ExitCode),
	}, nil
}

// Install is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterPlaybookServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
