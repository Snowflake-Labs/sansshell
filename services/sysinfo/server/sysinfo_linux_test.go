//go:build linux
// +build linux

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

package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
	conn    *grpc.ClientConn
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	lfs := &server{}
	lfs.Register(s)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

func TestUptime(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewSysInfoClient(conn)

	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)

	savedPath := getUptimeFilePath
	getUptimeFilePath = func() (string, error) {
		_, err := savedPath()
		if err != nil {
			return "", err
		}
		return f1.Name(), nil
	}

	for _, tc := range []struct {
		name        string
		fileContent string
		want        int64
		wantErr     bool
	}{
		{
			name:        "read normal /proc/uptime file",
			fileContent: "219070.82 1510410.71",
			want:        219070,
		},
		{
			name:    "Bad file - read empty proc/uptime file",
			wantErr: true,
		},
		{
			name:        "Bad file - /proc/uptime file contains bad content rather than numbers",
			fileContent: "abc",
			wantErr:     true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Truncate the file before the second write
			err = f1.Truncate(0)
			if err != nil {
				testutil.FatalOnErr("Failed to truncate the content of the temp file", err, t)
			}

			// Seek to the beginning of the file before the second write
			_, err = f1.Seek(0, 0)
			if err != nil {
				testutil.FatalOnErr("Failed to set the offset for next read/write", err, t)
			}

			// Write content to temp file
			_, err = f1.WriteString(tc.fileContent)
			if err != nil {
				testutil.FatalOnErr("Failed to write content to temp file", err, t)
			}

			resp, err := client.Uptime(ctx, &emptypb.Empty{})
			testutil.WantErr(tc.name, err, tc.wantErr, t)
			if tc.wantErr {
				t.Logf("%s: %v", tc.name, err)
				return
			}

			testutil.FatalOnErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			fmt.Println("got: ", resp.GetUptimeSeconds(), "want: ", tc.want)
			if got, want := resp.GetUptimeSeconds().Seconds, tc.want; got != want {
				t.Fatalf("uptime differ. Got %q Want %q", got, want)
			}
		})
	}

	// uptime is not supported in other OS, so an error should be raised
	getUptimeFilePath = func() (string, error) {
		return "", status.Errorf(codes.Unimplemented, "uptime is not supported")
	}
	for _, tc := range []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "uptime action not suported in other OS except Linux",
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Uptime(ctx, &emptypb.Empty{})
			testutil.WantErr(tc.name, err, tc.wantErr, t)
			if tc.wantErr {
				t.Logf("%s: %v", tc.name, err)
				return
			}
		})
	}

	// if `/proc/uptime` file doesn't exit in linux system, error should be raised
	getUptimeFilePath = func() (string, error) {
		return "/no-such-file-for-procuptime", nil
	}
	t.Cleanup(func() { getUptimeFilePath = savedPath })
	for _, tc := range []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "/proc/uptime not exit",
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Uptime(ctx, &emptypb.Empty{})
			testutil.WantErr(tc.name, err, tc.wantErr, t)
			if tc.wantErr {
				t.Logf("%s: %v", tc.name, err)
				return
			}
		})
	}

}
