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

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/Snowflake-Labs/sansshell/services/util"
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
}

func generateFDBMoveDataArgsImpl(req *pb.FDBMoveDataRequest) ([]string, error) {
	command := []string{FDBMoveOrchestrator}

	var args []string
	args = append(args, "--cluster", req.ClusterFile)
	args = append(args, "--tenant-group", req.TenantGroup)
	args = append(args, "--src-name", req.SourceCluster)
	args = append(args, "--dst-name", req.DestinationCluster)

	command = append(command, args...)
	return command, nil
}

func (s *fdbmovedata) FDBMoveData(ctx context.Context, req *pb.FDBMoveDataRequest) (*pb.FDBMoveDataResponse, error) {

	command, err := generateFDBMoveDataArgs(req)
	if err != nil {
		return nil, err
	}
	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error running fdbmovedata cmd (%+v): %v", command, err)
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		return nil, status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	resp := &pb.FDBMoveDataResponse{
		RetCode: int32(run.ExitCode),
		Stderr:  run.Stderr.Bytes(),
		Stdout:  run.Stdout.Bytes(),
	}
	return resp, nil
}

// Register is called to expose this handler to the gRPC server
func (s *fdbmovedata) Register(gs *grpc.Server) {
	pb.RegisterFDBMoveServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&fdbmovedata{})
}
