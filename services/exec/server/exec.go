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

	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/exec"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
)

// Metrics
const (
	execRunFailureCounterName = "actions_exec_run_failure"
	execRunFailureCounterDesc = "number of failures when performing exec.Run"
)

// server is used to implement the gRPC server
type server struct{}

// Run executes command and returns result
func (s *server) Run(ctx context.Context, req *pb.ExecRequest) (res *pb.ExecResponse, err error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	run, err := util.RunCommand(ctx, req.Command, req.Args)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(execRunFailureCounterName, execRunFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+execRunFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, execRunFailureCounterName, 1)
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+execRunFailureCounterName)
		}
		return nil, err
	}

	if run.Error != nil {
		errRegister := recorder.RegisterInt64Counter(execRunFailureCounterName, execRunFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+execRunFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, execRunFailureCounterName, 1)
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+execRunFailureCounterName)
		}
		return nil, run.Error
	}
	return &pb.ExecResponse{Stderr: run.Stderr.Bytes(), Stdout: run.Stdout.Bytes(), RetCode: int32(run.ExitCode)}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterExecServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
