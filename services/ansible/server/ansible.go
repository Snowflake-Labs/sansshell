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

// Package server implements the sansshell 'Ansible' service.
package server

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/ansible"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
)

var (
	// AnsiblePlaybookBin is the location to the ansible binary. Binding this to a flag is often useful.
	AnsiblePlaybookBin = "/usr/bin/ansible-playbook"

	// A test hook so we can take the args passed and transform them as needed.
	cmdArgsTransform = func(input []string) []string {
		return input
	}
)

// Metrics
var (
	ansibleRunFailureCounter = metrics.MetricDefinition{Name: "actions_ansible_run_failure",
		Description: "number of failures when performing ansible.Run"}
)

// server is used to implement the gRPC server
type server struct{}

var re = regexp.MustCompile("[^a-zA-Z0-9_/]+")

func (s *server) Run(ctx context.Context, req *pb.RunRequest) (*pb.RunReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	// Basic sanity checking up front.
	if AnsiblePlaybookBin == "" {
		recorder.CounterOrLog(ctx, ansibleRunFailureCounter, 1, attribute.String("reason", "not_implemented"))
		return nil, status.Error(codes.Unimplemented, "not implemented")
	}

	if req.Playbook == "" {
		recorder.CounterOrLog(ctx, ansibleRunFailureCounter, 1, attribute.String("reason", "missing_playbook"))
		return nil, status.Error(codes.InvalidArgument, "playbook path must be filled in")
	}
	if err := util.ValidPath(req.Playbook); err != nil {
		recorder.CounterOrLog(ctx, ansibleRunFailureCounter, 1, attribute.String("reason", "playbook_not_found"))
		return nil, err
	}

	// Make sure it's a valid file and nothing something which might be malicious like
	// /some/path && rm -rf /
	stat, err := os.Stat(req.Playbook)
	if err != nil || stat.IsDir() {
		recorder.CounterOrLog(ctx, ansibleRunFailureCounter, 1, attribute.String("reason", "playbook_not_found"))
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

	run, err := util.RunCommand(ctx, AnsiblePlaybookBin, cmdArgs)
	if err != nil {
		recorder.CounterOrLog(ctx, ansibleRunFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}
	if err := run.Error; err != nil {
		recorder.CounterOrLog(ctx, ansibleRunFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "command exited with error: %v (%d)\n%s", err, run.ExitCode, util.TrimString(run.Stderr.String()))
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
