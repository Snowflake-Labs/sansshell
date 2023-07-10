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
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	FDBServer string = "/usr/sbin/fdbserver" // uncomment that if fdbserver binary is in that location
	// FDBServer string
	// generateFDBServerArgs exists as a var for testing purposes
	generateFDBServerArgs = generateFDBServerArgsImpl
)

// Metrics
var (
	fdbserverFailureCounter = metrics.MetricDefinition{Name: "actions_fdbserver_failure",
		Description: "number of failures when performing fdbserver"}
)

type fdbserver struct {
}

func parseFDBServerCommand(req *pb.FDBServerCommand) (args []string, l []captureLogs, err error) {
	if req.Command == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "command must be filled in")
	}
	switch req.Command.(type) {
	case *pb.FDBServerCommand_Version:
		args, l, err = []string{"--version"}, nil, nil
	default:
		return nil, nil, status.Errorf(codes.InvalidArgument, "unknown type %T for command", req.Command)
	}
	return
}

func generateFDBServerArgsImpl(req *pb.FDBServerRequest) ([]string, error) {
	command := []string{FDBServer}

	var args []string
	for _, c := range req.Commands {
		newArgs, _, err := parseFDBServerCommand(c)
		if err != nil {
			return nil, err
		}
		args = append(args, newArgs...)
		if len(req.Commands) > 1 {
			args = append(args, ";")
		}
	}
	command = append(command, args...)
	return command, nil
}

func (s *fdbserver) FDBServer(ctx context.Context, req *pb.FDBServerRequest) (*pb.FDBServerResponse, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if len(req.Commands) == 0 {
		recorder.CounterOrLog(ctx, fdbcliFailureCounter, 1, attribute.String("reason", "missing_command"))
		return nil, status.Error(codes.InvalidArgument, "must fill in at least one command")
	}

	command, err := generateFDBServerArgs(req)
	if err != nil {
		recorder.CounterOrLog(ctx, fdbserverFailureCounter, 1, attribute.String("reason", "generate_args_err"))
		return nil, err
	}

	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, fdbserverFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error running fdbserver cmd (%+v): %v", command, err)
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, fdbserverFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	resp := &pb.FDBServerResponse{
		RetCode: int32(run.ExitCode),
		Stderr:  run.Stderr.Bytes(),
		Stdout:  run.Stdout.Bytes(),
	}
	return resp, nil
}

// Register is called to expose this handler to the gRPC server
func (s *fdbserver) Register(gs *grpc.Server) {
	pb.RegisterServerServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&fdbserver{})
}
