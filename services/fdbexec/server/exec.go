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

// Package server implements the sansshell 'FdbExec' service.
package server

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/fdbexec"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
)

// Metrics
var (
	fdbexecRunFailureCounter = metrics.MetricDefinition{Name: "actions_fdbexec_run_failure",
		Description: "number of failures when performing fdbexec.Run"}
)

// server is used to implement the gRPC server
type server struct{}

// loadFdbEnvFile loads environment variables from /etc/fdb.env if it exists
func loadFdbEnvFile() ([]string, error) {
	// Check if /etc/fdb.env exists
	if _, err := os.Stat("/etc/fdb.env"); os.IsNotExist(err) {
		return nil, nil
	}

	// Open the environment file
	file, err := os.Open("/etc/fdb.env")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var envVars []string

	// Read each line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) > 1 && (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		// Add the environment variable
		envVars = append(envVars, key+"="+value)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return envVars, nil
}

// Run executes command and returns result
func (s *server) Run(ctx context.Context, req *pb.FdbExecRequest) (res *pb.FdbExecResponse, err error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	var opts []util.Option
	if req.User != "" {
		uid, gid, err := resolveUser(req.User)
		if err != nil {
			return nil, err
		}
		opts = append(opts, util.CommandUser(uint32(uid)))
		opts = append(opts, util.CommandGroup(uint32(gid)))
	}

	// Load environment variables from /etc/fdb.env
	envVars, err := loadFdbEnvFile()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load environment variables: %v", err)
	}

	// Add environment variables as options
	if envVars != nil {
		for _, env := range envVars {
			opts = append(opts, util.EnvVar(env))
		}
	}

	run, err := util.RunCommand(ctx, req.Command, req.Args, opts...)
	if err != nil {
		recorder.CounterOrLog(ctx, fdbexecRunFailureCounter, 1)
		return nil, err
	}

	if run.Error != nil {
		recorder.CounterOrLog(ctx, fdbexecRunFailureCounter, 1)
		return nil, run.Error
	}
	return &pb.FdbExecResponse{Stderr: run.Stderr.Bytes(), Stdout: run.Stdout.Bytes(), RetCode: int32(run.ExitCode)}, nil
}

// StreamingRun executes command and returns a stream of results
func (s *server) StreamingRun(req *pb.FdbExecRequest, stream pb.FdbExec_StreamingRunServer) error {
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
	if req.User != "" {
		uid, gid, err := resolveUser(req.User)
		if err != nil {
			return err
		}

		// Set uid/gid if needed for the sub-process to run under.
		// Only do this if it's different than our current ones since
		// attempting to setuid/gid() to even your current values is EPERM.
		if uid != uint32(os.Geteuid()) || gid != uint32(os.Getgid()) {
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uid,
					Gid: gid,
				},
			}
		}
	}

	// Set default empty environment
	cmd.Env = []string{}

	// Load environment variables from /etc/fdb.env
	envVars, err := loadFdbEnvFile()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to load environment variables: %v", err)
	}

	// Add environment variables to cmd.Env
	if envVars != nil {
		cmd.Env = append(cmd.Env, envVars...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

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
			if err := stream.Send(&pb.FdbExecResponse{Stderr: buf[:n]}); err != nil {
				recorder.CounterOrLog(ctx, fdbexecRunFailureCounter, 1)
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
			recorder.CounterOrLog(ctx, fdbexecRunFailureCounter, 1)
			return err
		}
		if err := stream.Send(&pb.FdbExecResponse{Stdout: buf[:n]}); err != nil {
			return err
		}
	}

	// If we've gotten here, stdout has been closed and the command is over
	err = cmd.Wait()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stream.Send(&pb.FdbExecResponse{RetCode: int32(exitErr.ExitCode())})
	}
	return err
}

// resolveUser retruns uid and gid of provided username.
func resolveUser(username string) (uint32, uint32, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return 0, 0, status.Errorf(codes.InvalidArgument, "user '%s' not found:\n%v", username, err)
	}
	// This will work only on POSIX (Windows has non-decimal uids) yet these are our targets.
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, status.Errorf(codes.Internal, "'%s' user's uid %s failed to convert to numeric value:\n%v", username, u.Uid, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, status.Errorf(codes.Internal, "'%s' user's gid %s failed to convert to numeric value:\n%v", username, u.Gid, err)
	}
	return uint32(uid), uint32(gid), nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterFdbExecServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
