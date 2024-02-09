/* Copyright (c) 2024 Snowflake Inc. All rights reserved.

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

// Package server implements the sansshell 'tcpoverrpc' service.
package server

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/tcpoverrpc"
)

// Server is used to implement the gRPC Server
type server struct{}

const defaultTimeoutSeconds = 10

func (s *server) Ok(ctx context.Context, req *pb.HostTCPRequest) (*emptypb.Empty, error) {
	if req.Hostname == "" {
		return nil, fmt.Errorf("hostname cannot be empty")
	}
	host := net.JoinHostPort(req.Hostname, strconv.Itoa(int(req.Port)))

	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if req.TimeoutSeconds <= 0 {
		timeout = defaultTimeoutSeconds * time.Second
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("failed to dial tcp %s: %v", host, err)
	}
	conn.Close()

	return &emptypb.Empty{}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterTCPOverRPCServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
