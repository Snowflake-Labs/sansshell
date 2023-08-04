/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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

package server

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"sync"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// fdb_move_orchestrator binary location
	FDBMoveOrchestrator     string
	generateFDBMoveDataArgs = generateFDBMoveDataArgsImpl
)

type fdbmovedata struct {
	mu  sync.Mutex
	id  int64
	cmd *exec.Cmd
}

func generateFDBMoveDataArgsImpl(req *pb.FDBMoveDataCopyRequest) ([]string, error) {
	command := []string{FDBMoveOrchestrator}

	var args []string
	args = append(args, "--cluster", req.ClusterFile)
	args = append(args, "--tenant-group", req.TenantGroup)
	args = append(args, "--src-name", req.SourceCluster)
	args = append(args, "--dst-name", req.DestinationCluster)
	args = append(args, "--num-procs", fmt.Sprintf("%d", req.NumProcs))

	command = append(command, args...)
	return command, nil
}

func (s *fdbmovedata) FDBMoveDataCopy(ctx context.Context, req *pb.FDBMoveDataCopyRequest) (*pb.FDBMoveDataCopyResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logger := logr.FromContextOrDiscard(ctx)
	// The sansshell server should only run one copy command at a time
	if !(s.cmd == nil) {
		logger.Info("existing command already running. returning early")
		earlyresp := &pb.FDBMoveDataCopyResponse{
			Id: s.id,
		}
		return earlyresp, nil
	}
	s.id = rand.Int63()
	command, err := generateFDBMoveDataArgs(req)
	if err != nil {
		return nil, err
	}

	// Add env vars for python and python bindings
	cmd := exec.Command(command[0], command[1:]...)
	// TODO: change to finalized locations when ready
	cmd.Env = append(cmd.Environ(), "PYTHONPATH=/home/teleport-jfu/python")
	cmd.Env = append(cmd.Environ(), "PATH=/opt/rh/rh-python36/root/bin")

	logger.Info("executing local command", "cmd", cmd.String())
	s.cmd = cmd
	err = s.cmd.Start()
	if err != nil {
		s.cmd = nil
		return nil, status.Errorf(codes.Internal, "error running fdbmovedata cmd (%+v): %v", command, err)
	}

	resp := &pb.FDBMoveDataCopyResponse{
		Id: s.id,
	}
	return resp, nil
}

func (s *fdbmovedata) FDBMoveDataWait(ctx context.Context, req *pb.FDBMoveDataWaitRequest) (*pb.FDBMoveDataWaitResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !(req.Id == s.id) {
		return nil, status.Errorf(codes.Internal, "Provided ID %d does not match stored ID %d", req.Id, s.id)
	}
	if s.cmd == nil {
		return nil, status.Errorf(codes.Internal, "No command running on the server")
	}

	run := &util.CommandRun{
		Stdout: util.NewLimitedBuffer(util.DefRunBufLimit),
		Stderr: util.NewLimitedBuffer(util.DefRunBufLimit),
	}
	s.cmd.Stdout = run.Stdout
	s.cmd.Stderr = run.Stderr
	s.cmd.Stdin = nil

	err := s.cmd.Wait()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error waiting for fdbmovedata cmd: %v", err)
	}

	resp := &pb.FDBMoveDataWaitResponse{
		Stdout:  run.Stdout.Bytes(),
		Stderr:  run.Stderr.Bytes(),
		RetCode: int32(s.cmd.ProcessState.ExitCode()),
	}
	// clear the cmd to allow another call
	s.cmd = nil
	s.id = 0
	// 	// Send stderr asynchronously
	// 	go func() {
	// 		for {
	// 			buf := make([]byte, 1024)
	// 			n, err := stderr.Read(buf)
	// 			if err != nil {
	// 				return
	// 			}
	// 			if err := stream.Send(&pb.ExecResponse{Stderr: buf[:n]}); err != nil {
	// 				recorder.CounterOrLog(ctx, execRunFailureCounter, 1)
	// 				return
	// 			}
	// 		}
	// 	}()

	// 	// Send stdout synchronously
	// 	for {
	// 		buf := make([]byte, 1024)
	// 		n, err := stdout.Read(buf)
	// 		if err == io.EOF {
	// 			break
	// 		} else if err != nil {
	// 			recorder.CounterOrLog(ctx, execRunFailureCounter, 1)
	// 			return err
	// 		}
	// 		if err := stream.Send(&pb.ExecResponse{Stdout: buf[:n]}); err != nil {
	// 			return err
	// 		}
	// 	}

	return resp, nil
}

// Register is called to expose this handler to the gRPC server
func (s *fdbmovedata) Register(gs *grpc.Server) {
	pb.RegisterFDBMoveServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&fdbmovedata{})
}
