//go:build !linux
// +build !linux

/*
Copyright (c) 2023 Snowflake Inc. All rights reserved.

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
	"time"

	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var getUptime = func() (time.Duration, error) {
	return 0, status.Errorf(codes.Unimplemented, "uptime is not supported")
}

var getKernelMessages = func(time.Duration, <-chan struct{}) ([]*pb.DmsgRecord, error) {
	return nil, status.Errorf(codes.Unimplemented, "dmesg is not supported")
}

var getJournalRecordsAndSend = func(ctx context.Context, req *pb.JournalRequest, stream pb.SysInfo_JournalServer) error {
	return status.Errorf(codes.Unimplemented, "journal is not supported")
}
