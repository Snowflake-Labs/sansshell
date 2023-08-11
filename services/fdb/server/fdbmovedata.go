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
	"io"
	"math/rand"
	"os/exec"
	"sync"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// fdb_move_orchestrator binary location
	// Will be checked into:
	// https://github.com/apple/foundationdb/tree/snowflake/release-71.3/bindings/python/fdb/fdb_move_orchestrator.py
	FDBMoveOrchestrator     string
	generateFDBMoveDataArgs = generateFDBMoveDataArgsImpl
)

type fdbmovedata struct {
	mu     sync.Mutex
	id     int64
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
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
	lockSuccess := s.mu.TryLock()
	if !(lockSuccess) {
		return nil, status.Errorf(codes.Internal, "Copy or Wait command already running")
	}
	defer s.mu.Unlock()
	logger := logr.FromContextOrDiscard(ctx)
	// The sansshell server should only run one copy command at a time
	if !(s.cmd == nil) {
		logger.Info("existing command already running. returning early")
		logger.Info("command details", "cmd", s.cmd.String())
		logger.Info("command running with id", "id", s.id)
		earlyresp := &pb.FDBMoveDataCopyResponse{
			Id:       s.id,
			Existing: true,
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
	cmd.Env = append(cmd.Environ(), "PYTHONPATH=/opt/rh/rh-python36/root/lib/python3.6/site-packages/")
	cmd.Env = append(cmd.Environ(), "PATH=/opt/rh/rh-python36/root/bin")
	cmd.Env = append(cmd.Environ(), "PYTHONUNBUFFERED=true")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	logger.Info("executing local command", "cmd", cmd.String())
	logger.Info("command running with id", "id", s.id)
	s.cmd = cmd
	s.stdout = stdout
	s.stderr = stderr
	err = s.cmd.Start()
	if err != nil {
		s.cmd = nil
		s.id = 0
		return nil, status.Errorf(codes.Internal, "error running fdbmovedata cmd (%+v): %v", command, err)
	}

	resp := &pb.FDBMoveDataCopyResponse{
		Id:       s.id,
		Existing: false,
	}
	return resp, nil
}

func (s *fdbmovedata) FDBMoveDataWait(req *pb.FDBMoveDataWaitRequest, stream pb.FDBMove_FDBMoveDataWaitServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !(req.Id == s.id) {
		return status.Errorf(codes.Internal, "Provided ID %d does not match stored ID %d", req.Id, s.id)
	}
	if s.cmd == nil {
		return status.Errorf(codes.Internal, "No command running on the server")
	}
	wg := &sync.WaitGroup{}

	// Send stderr asynchronously
	stderr := s.stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			buf := make([]byte, 1024)
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			if err := stream.Send(&pb.FDBMoveDataWaitResponse{Stderr: buf[:n]}); err != nil {
				return
			}
		}
	}()

	// Send stdout asynchronously
	stdout := s.stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			buf := make([]byte, 1024)
			n, err := stdout.Read(buf)
			if err != nil {
				return
			}
			if err := stream.Send(&pb.FDBMoveDataWaitResponse{Stdout: buf[:n]}); err != nil {
				return
			}
		}
	}()
	wg.Wait()
	err := s.cmd.Wait()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stream.Send(&pb.FDBMoveDataWaitResponse{RetCode: int32(exitErr.ExitCode())})
	}
	// clear the cmd to allow another call
	s.cmd = nil
	s.id = 0
	return err
}

// Register is called to expose this handler to the gRPC server
func (s *fdbmovedata) Register(gs *grpc.Server) {
	pb.RegisterFDBMoveServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&fdbmovedata{})
}
