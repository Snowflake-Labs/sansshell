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
	"errors"
	"os"
	"strings"

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
	// FDBBackup binary location
	FDBBackup string = "/usr/sbin/fdbbackup"

	// FDBBackupEnvFile is a file with newline-separated environment variables to set before running fdbbackup
	FDBBackupEnvFile string = "/etc/fdb.env"
)

// Metrics
var (
	fdbbackupFailureCounter = metrics.MetricDefinition{Name: "actions_fdbbackup_failure",
		Description: "number of failures when performing fdbbackup"}
)

type fdbbackup struct{}

func runFDBBackupCommand(ctx context.Context, args []string) (*pb.FDBBackupResponse, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	command := []string{FDBBackup}
	command = append(command, args...)

	var opts []util.Option
	// Add env vars from file if it exists
	if _, err := os.Stat(FDBBackupEnvFile); !errors.Is(err, os.ErrNotExist) {
		content, err := os.ReadFile(FDBBackupEnvFile)
		if err != nil {
			return nil, err
		}
		for _, l := range strings.Split(string(content), "\n") {
			if l != "" {
				opts = append(opts, util.EnvVar(l))
			}
		}
	}

	run, err := util.RunCommand(ctx, command[0], command[1:], opts...)
	if err != nil {
		recorder.CounterOrLog(ctx, fdbbackupFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error running fdbbackup cmd (%+v): %v", command, err)
	}

	resp := &pb.FDBBackupResponse{
		RetCode: int32(run.ExitCode),
		Stderr:  run.Stderr.Bytes(),
		Stdout:  run.Stdout.Bytes(),
	}

	return resp, nil
}

func (s *fdbbackup) FDBBackupStatus(ctx context.Context, req *pb.FDBBackupStatusRequest) (*pb.FDBBackupResponse, error) {
	var args []string

	args = append(args, "status")

	if req.ClusterFile != "" {
		args = append(args, "-C", req.ClusterFile)
	}

	if req.BackupUrl != "" {
		args = append(args, "--blob_url", req.BackupUrl)
	}

	return runFDBBackupCommand(ctx, args)
}

func (s *fdbbackup) FDBBackupAbort(ctx context.Context, req *pb.FDBBackupAbortRequest) (*pb.FDBBackupResponse, error) {
	var args []string

	args = append(args, "abort")

	if req.ClusterFile != "" {
		args = append(args, "-C", req.ClusterFile)
	}

	if req.BackupUrl != "" {
		args = append(args, "--blob_url", req.BackupUrl)
	}

	return runFDBBackupCommand(ctx, args)
}

func (s *fdbbackup) FDBBackupStart(ctx context.Context, req *pb.FDBBackupStartRequest) (*pb.FDBBackupResponse, error) {
	var args []string

	args = append(args, "start")

	if req.ClusterFile != "" {
		args = append(args, "-C", req.ClusterFile)
	}

	if req.BackupUrl != "" {
		args = append(args, "--blob_url", req.BackupUrl)
	}

	if req.Snapshot {
		args = append(args, "--snapshot")
	}

	if req.Tag != "" {
		args = append(args, "--tag", req.Tag)
	}

	return runFDBBackupCommand(ctx, args)
}

func (s *fdbbackup) FDBBackupDescribe(ctx context.Context, req *pb.FDBBackupDescribeRequest) (*pb.FDBBackupResponse, error) {
	var args []string

	args = append(args, "describe")

	if req.ClusterFile != "" {
		args = append(args, "-C", req.ClusterFile)
	}

	if req.BackupUrl != "" {
		args = append(args, "--blob_url", req.BackupUrl)
	}

	return runFDBBackupCommand(ctx, args)
}

func (s *fdbbackup) FDBBackupExpire(ctx context.Context, req *pb.FDBBackupExpireRequest) (*pb.FDBBackupResponse, error) {
	var args []string

	args = append(args, "expire")

	if req.ClusterFile != "" {
		args = append(args, "-C", req.ClusterFile)
	}

	if req.BackupUrl != "" {
		args = append(args, "--blob_url", req.BackupUrl)
	}

	if req.Version != "" {
		args = append(args, "--version", req.Version)
	}

	return runFDBBackupCommand(ctx, args)
}

func (s *fdbbackup) FDBBackupPause(ctx context.Context, req *pb.FDBBackupPauseRequest) (*pb.FDBBackupResponse, error) {
	var args []string

	args = append(args, "pause")

	if req.ClusterFile != "" {
		args = append(args, "-C", req.ClusterFile)
	}

	if req.BackupUrl != "" {
		args = append(args, "--blob_url", req.BackupUrl)
	}

	if req.Tag != "" {
		args = append(args, "--tag", req.Tag)
	}

	return runFDBBackupCommand(ctx, args)
}

func (s *fdbbackup) FDBBackupResume(ctx context.Context, req *pb.FDBBackupResumeRequest) (*pb.FDBBackupResponse, error) {
	var args []string

	args = append(args, "resume")

	if req.ClusterFile != "" {
		args = append(args, "-C", req.ClusterFile)
	}

	if req.BackupUrl != "" {
		args = append(args, "--blob_url", req.BackupUrl)
	}

	if req.Tag != "" {
		args = append(args, "--tag", req.Tag)
	}

	return runFDBBackupCommand(ctx, args)
}

// Register is called to expose this handler to the gRPC server
func (s *fdbbackup) Register(gs *grpc.Server) {
	pb.RegisterFDBBackupServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&fdbbackup{})
}
