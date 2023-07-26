//go:build linux
// +build linux

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
	"fmt"
	"time"

	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"github.com/euank/go-kmsg-parser/v2/kmsgparser"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// for testing
var getKmsgParser = func() (kmsgparser.Parser, error) {
	return kmsgparser.NewParser()
}

var getUptime = func() (time.Duration, error) {
	sysinfo := &unix.Sysinfo_t{}
	if err := unix.Sysinfo(sysinfo); err != nil {
		fmt.Println(err)
		return 0, status.Errorf(codes.Internal, "err in get the system info from unix")
	}
	uptime := time.Duration(sysinfo.Uptime) * time.Second
	return uptime, nil
}

// Based on: https://pkg.go.dev/github.com/euank/go-kmsg-parser
// kmsg-parser only allows us to read message from /dev/kmsg in blocking way
// we set 2 seconds timeout to explicitly close the channel
// If the package release new feature to support non-blocking read, we can
// make corresding changes below to get rid of hard code timeout setting
var getKernelMessages = func() ([]*pb.DmsgRecord, error) {
	parser, err := getKmsgParser()
	if err != nil {
		return nil, err
	}

	var records []*pb.DmsgRecord
	messages := parser.Parse()
	done := false
	for !done {
		select {
		case msg, ok := <-messages:
			if !ok {
				done = true
			}
			// process the message
			records = append(records, &pb.DmsgRecord{
				Message: msg.Message,
				Time:    timestamppb.New(msg.Timestamp),
			})
		case <-time.After(2 * time.Second):
			parser.Close()
			done = true
		}
	}
	return records, nil
}
