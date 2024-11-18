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

package server

import (
	"context"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	localfileShredFailureCounter = metrics.MetricDefinition{
		Name:        "actions_localfile_shred_failure",
		Description: "number of failures when performing localfile.Shred",
	}
)

func (s *server) Shred(ctx context.Context, req *pb.ShredRequest) (*emptypb.Empty, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	if err := util.ValidPath(req.Filename); err != nil {
		recorder.CounterOrLog(ctx, localfileShredFailureCounter, 1, attribute.String("reason", "invalid_path"))
		return nil, err
	}

	shredPath, err := util.Which("shred")
	if err != nil {
		recorder.CounterOrLog(ctx, localfileShredFailureCounter, 1, attribute.String("reason", "shred_not_found"))
		return nil, err
	}
	args := make([]string, 0, 4)

	if req.Force {
		args = append(args, "-f")
	}

	if req.Zero {
		args = append(args, "-z")
	}

	if req.Remove {
		args = append(args, "-u")
	}

	args = append(args, req.Filename)

	// we want to fail as soon as possible, if the file is unreadable, opened by someone else or something else happens
	// to it while we're trying to shred it, we might as well just fail during command execution
	r, err := util.RunCommand(ctx, shredPath, args)
	if err != nil {
		recorder.CounterOrLog(ctx, localfileShredFailureCounter, 1, attribute.String("reason", "shred_execution_failed"))
		return nil, status.Errorf(codes.Internal, "shred command run failed: %v", err)
	}
	if r.ExitCode != 0 {
		recorder.CounterOrLog(ctx, localfileShredFailureCounter, 1, attribute.String("reason", "shred_execution_failed"))
		return nil, status.Errorf(codes.Internal, "error shredding file %s: %s", req.Filename, util.TrimString(r.Stderr.String()))
	}

	return &emptypb.Empty{}, nil
}
