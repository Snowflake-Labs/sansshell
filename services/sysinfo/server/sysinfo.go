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
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	sysinfoDmesgFailureCounter = metrics.MetricDefinition{Name: "actions_sysinfo_dmesg_failure",
		Description: "number of failures when performing sysinfo.Dmesg"}
	sysinfoJournalFailureCounter = metrics.MetricDefinition{Name: "actions_sysinfo_journal_failure",
		Description: "number of failures when performing sysinfo.Journal"}
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

func (s *server) Dmesg(req *pb.DmesgRequest, stream pb.SysInfo_DmesgServer) error {
	ctx := stream.Context()
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if req.Grep == "" && (req.IgnoreCase || req.InvertMatch) {
		return status.Error(codes.InvalidArgument, "must provide grep argument before setting ignore_case or invert_match")
	}

	records, err := getKernelMessages()
	if err != nil {
		recorder.CounterOrLog(ctx, sysinfoDmesgFailureCounter, 1, attribute.String("reason", "get kernel message error"))
		return status.Errorf(codes.InvalidArgument, "can't get kernel message %v", err)
	}
	// grep the messages we want
	if req.Grep != "" {
		var newRecords []*pb.DmsgRecord
		regex := req.Grep
		if req.IgnoreCase {
			regex = `(?i)` + regex
		}

		isInvert := req.InvertMatch
		pattern := regexp.MustCompile(regex)
		for _, record := range records {
			// Two cases are what we want
			// case 1: isInvert = false and match = true (pattern matched)
			// case 2: isInvert = true and match = false (pattern doesn't match)
			if match := pattern.MatchString(record.Message); match != isInvert {
				newRecords = append(newRecords, record)
			}
		}
		records = newRecords
	}

	// tail the last n lines
	// there is no way to tail number of messages more than initial records
	if req.TailLines > int32(len(records)) {
		req.TailLines = int32(len(records))
	}
	// negative number means disables the tail feature
	if req.TailLines > 0 {
		records = records[len(records)-int(req.TailLines):]
	}
	for _, r := range records {
		if err := stream.Send(&pb.DmesgReply{Record: r}); err != nil {
			recorder.CounterOrLog(ctx, sysinfoDmesgFailureCounter, 1, attribute.String("reason", "stream_send_err"))
			return status.Errorf(codes.Internal, "dmesg: send error %v", err)
		}
	}
	return nil
}

func (s *server) Journal(req *pb.JournalRequest, stream pb.SysInfo_JournalServer) error {
	ctx := stream.Context()
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	// currently output can only be json or json-pretty
	if req.Output != "" && req.Output != "json" && req.Output != "json-pretty" {
		recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "invalid_args"))
		return status.Errorf(codes.InvalidArgument, "cannot set output to other formats unless json or json-pretty")
	}

	err := getJournalRecordsAndSend(ctx, req, stream)
	if err != nil {
		recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "get_journal_and_send_err"))
		return err
	}

	// for _, record := range records {
	// 	if err := stream.Send(record); err != nil {
	// 		recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "stream_send_err"))
	// 		return status.Errorf(codes.Internal, "journal: send error %v", err)
	// 	}
	// }
	return nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterSysInfoServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
