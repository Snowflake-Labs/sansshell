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

// Package server implements the sansshell 'SysInfo' service.
package server

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.opentelemetry.io/otel/attribute"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
)

// Metrics
var (
	sysinfoUptimeFailureCounter = metrics.MetricDefinition{Name: "actions_sysinfo_uptime_failure",
		Description: "number of failures when performing sysinfo.Uptime"}
)

// server is used to implement the gRPC server
type server struct {
}

func (s *server) Uptime(ctx context.Context, in *emptypb.Empty) (*pb.UptimeReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	uptime, err := getUptime()
	if err != nil {
		recorder.CounterOrLog(ctx, sysinfoUptimeFailureCounter, 1, attribute.String("reason", "get_uptime_err"))
		return nil, err
	}

	uptimeSecond := durationpb.New(uptime)
	reply := &pb.UptimeReply{
		UptimeSeconds: uptimeSecond,
	}
	return reply, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterSysInfoServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
