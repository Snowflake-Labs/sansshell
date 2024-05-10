//go:build !linux
// +build !linux

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

	pb "github.com/Snowflake-Labs/sansshell/services/service"
	"github.com/coreos/go-systemd/v22/dbus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func GetServiceStatus(ctx context.Context, conn *dbus.Conn, serviceName string, displayTimestamp bool) (*pb.StatusReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "check service status is not supported in OS other than linux")
}

func createServer() pb.ServiceServer {
	return pb.UnimplementedServiceServer{}
}
