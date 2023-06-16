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
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
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
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	file, err := getUptimeFilePath()
	if err != nil {
		recorder.CounterOrLog(ctx, sysinfoUptimeFailureCounter, 1, attribute.String("reason", "get_uptime_file_err"))
		return nil, err
	}

	f, err := os.Open(file)
	if err != nil {
		recorder.CounterOrLog(ctx, sysinfoUptimeFailureCounter, 1, attribute.String("reason", "open_err"))
		return nil, status.Errorf(codes.Internal, "can't open file %s: %v", file, err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			recorder.CounterOrLog(ctx, sysinfoUptimeFailureCounter, 1, attribute.String("reason", "close_err"))
			logger.Error(err, "file.Close()", "file", file)
		}
	}()

	// read bytes from file and append to content
	buf := make([]byte, util.StreamingChunkSize)
	content := ""
	for {
		n, err := f.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			recorder.CounterOrLog(ctx, sysinfoUptimeFailureCounter, 1, attribute.String("reason", "read_err"))
			return nil, status.Errorf(codes.Internal, "can't read file %s: %v", file, err)
		}
		content += string(buf[:n])
	}
	// trim the string of content
	contentTrim := strings.TrimSpace(content)
	if len(contentTrim) == 0 {
		return nil, status.Errorf(codes.Internal, "the file %s doesn't contain any uptime info", file)
	}
	words := strings.Split(contentTrim, " ")

	uptimeFloat, err := strconv.ParseFloat(words[0], 64)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot convert the time from string to float")
	}
	uptimeSecond := durationpb.New(time.Duration(uptimeFloat) * time.Second)
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
