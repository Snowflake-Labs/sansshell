package server

import (
	"context"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/exec"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"google.golang.org/grpc"
)

// server is used to implement the gRPC server
type server struct{}

// Run executes command and returns result
func (s *server) Run(ctx context.Context, req *pb.ExecRequest) (res *pb.ExecResponse, err error) {
	run, err := util.RunCommand(ctx, req.Command, req.Args)
	if err != nil {
		return nil, err
	}

	if err := run.Error; err != nil {
		return &pb.ExecResponse{
			Stdout:  run.Stdout.Bytes(),
			Stderr:  run.Stderr.Bytes(),
			RetCode: int32(run.ExitCode),
		}, nil
	}

	return &pb.ExecResponse{Stderr: run.Stderr.Bytes(), Stdout: run.Stdout.Bytes(), RetCode: 0}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterExecServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
