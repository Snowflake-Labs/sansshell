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

// Package server implements the sansshell 'Power' service.
package server

import (
	"context"
	"regexp"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/power"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	powerRebootFailureCounter = metrics.MetricDefinition{Name: "actions_power_reboot_failure",
		Description: "number of failures when performing power.Reboot"}

	HoursMinutesRe = regexp.MustCompile(`^\d{2}:\d{2}$`)
	PlusMinutesRe  = regexp.MustCompile(`^\+\d+$`)
)

func validateWhenField(value string) error {
	if len(value) == 0 {
		return status.Errorf(codes.InvalidArgument, "%s must be filled in", "when")
	}
	if value == "now" {
		return nil
	}
	if HoursMinutesRe.MatchString(value) {
		return nil
	}
	if PlusMinutesRe.MatchString(value) {
		return nil
	}

	return status.Errorf(codes.InvalidArgument, "%s is not a valid time format", "when")
}

type server struct{}

func (s *server) Reboot(ctx context.Context, req *pb.RebootRequest) (*emptypb.Empty, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if err := validateWhenField(req.When); err != nil {
		recorder.CounterOrLog(ctx, powerRebootFailureCounter, 1, attribute.String("reason", "invalid_when"))
		return nil, err
	}

	command, err := generateReboot(req)
	if err != nil {
		recorder.CounterOrLog(ctx, powerRebootFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
		return nil, err
	}

	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, powerRebootFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, powerRebootFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	return &emptypb.Empty{}, nil
}

func (s *server) Register(gs *grpc.Server) {
	pb.RegisterPowerServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
