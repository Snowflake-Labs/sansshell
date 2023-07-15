/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

// Package server implements the sansshell 'Exec' service.
package server

import (
	"context"
	"io"
	"os/exec"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/exec"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
)

// Metrics
var (
	execRunFailureCounter = metrics.MetricDefinition{Name: "actions_exec_run_failure",
		Description: "number of failures when performing exec.Run"}
)

// server is used to implement the gRPC server
type server struct{}

// Run executes command and returns result
func (s *server) Run(ctx context.Context, req *pb.ExecRequest) (res *pb.ExecResponse, err error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	run, err := util.RunCommand(ctx, req.Command, req.Args)
	if err != nil {
		recorder.CounterOrLog(ctx, execRunFailureCounter, 1)
		return nil, err
	}

	if run.Error != nil {
		recorder.CounterOrLog(ctx, execRunFailureCounter, 1)
		return nil, run.Error
	}
	return &pb.ExecResponse{Stderr: run.Stderr.Bytes(), Stdout: run.Stdout.Bytes(), RetCode: int32(run.ExitCode)}, nil
}

// StreamingRun executes command and returns a stream of results
func (s *server) StreamingRun(req *pb.ExecRequest, stream pb.Exec_StreamingRunServer) error {
	ctx := stream.Context()
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	// We can't use util.RunCommand because it runs the command synchronously, so we
	// need to do input validation normally performed by it.
	if !filepath.IsAbs(req.Command) {
		return status.Errorf(codes.InvalidArgument, "%s is not an absolute path", req.Command)
	}
	if req.Command != filepath.Clean(req.Command) {
		return status.Errorf(codes.InvalidArgument, "%s is not a clean path", req.Command)
	}

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Env = []string{}
	if err := cmd.Start(); err != nil {
		return err
	}

	// Send stderr asynchronously
	go func() {
		for {
			buf := make([]byte, 1024)
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			if err := stream.Send(&pb.ExecResponse{Stderr: buf[:n]}); err != nil {
				recorder.CounterOrLog(ctx, execRunFailureCounter, 1)
				return
			}
		}
	}()

	// Send stdout synchronously
	for {
		buf := make([]byte, 1024)
		n, err := stdout.Read(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			recorder.CounterOrLog(ctx, execRunFailureCounter, 1)
			return err
		}
		if err := stream.Send(&pb.ExecResponse{Stdout: buf[:n]}); err != nil {
			return err
		}
	}

	// If we've gotten here, stdout has been closed and the command is over
	err = cmd.Wait()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stream.Send(&pb.ExecResponse{RetCode: int32(exitErr.ExitCode())})
	}
	return err
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterExecServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
