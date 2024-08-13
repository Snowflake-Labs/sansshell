/* Copyright (c) 2022 Snowflake Inc. All rights reserved.

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

// Package server implements the sansshell 'Network' service.
package server

import (
	"context"
	"github.com/Snowflake-Labs/sansshell/services/util/validator"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/network"
	app "github.com/Snowflake-Labs/sansshell/services/network/server/application"
	infraOutput "github.com/Snowflake-Labs/sansshell/services/network/server/infrastructure/output"
)

// Server is used to implement the gRPC Server
type server struct {
	tcpCheckUsecase        app.TCPCheckUsecase
	tcpCheckFailureCounter metrics.MetricDefinition
}

func (s *server) TCPCheck(ctx context.Context, req *pb.TCPCheckRequest) (*pb.TCPCheckReply, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	hostname := req.GetHostname()
	rawPort := req.GetPort()
	timeout := req.GetTimeout().AsDuration()

	if err := validator.IsValidPortUint32(rawPort); err != nil {
		logger.Error(err, "Invalid port value")
		recorder.CounterOrLog(ctx, s.tcpCheckFailureCounter, 1)
		return nil, status.Errorf(codes.Internal, "Invalid port value: %d", rawPort)
	}
	port := uint8(rawPort)

	usecase := app.NewTCPCheckUsecase(&infraOutput.TCPClient{})

	result, err := usecase.Run(ctx, hostname, port, timeout)
	if err != nil {
		logger.Error(err, "Unexpected error")
		recorder.CounterOrLog(ctx, s.tcpCheckFailureCounter, 1)
		return nil, status.Errorf(codes.Internal, "Internal server error")
	}

	reply := &pb.TCPCheckReply{
		Ok:         result.IsOk,
		FailReason: result.FailReason,
	}

	return reply, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterNetworkServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{
		tcpCheckUsecase: app.NewTCPCheckUsecase(&infraOutput.TCPClient{}),
		tcpCheckFailureCounter: metrics.MetricDefinition{
			Name:        "actions_network_tcpcheck_failure",
			Description: "number of failures when performing network.TCPCheck",
		},
	})
}
