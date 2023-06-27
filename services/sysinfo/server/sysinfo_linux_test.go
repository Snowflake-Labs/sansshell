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
	"time"

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

	// test the unix.Sysinfo() returns non-negative uptime
	for _, tc := range []struct {
		name    string
		wantErr bool
	}{
		{
			name: "getUptime() function should return uptime larger than or equal to 0",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Uptime(ctx, &emptypb.Empty{})
			testutil.FatalOnErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			uptime := resp.GetUptimeSeconds().Seconds
			if uptime <= 0 {
				t.Fatalf("getUptime() returned an invalid uptime: %v", uptime)
			}
		})
	}

	savedUptime := getUptime
	getUptime = func() (time.Duration, error) {
		_, err := savedUptime()
		if err != nil {
			return 0, err
		}
		return 500 * time.Second, nil
	}

	for _, tc := range []struct {
		name    string
		want    int64
		wantErr bool
	}{
		{
			name: "get system uptime 500 seconds",
			want: 500,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Uptime(ctx, &emptypb.Empty{})
			testutil.FatalOnErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			fmt.Println("got: ", resp.GetUptimeSeconds(), "want: ", tc.want)
			if got, want := resp.GetUptimeSeconds().Seconds, tc.want; got != want {
				t.Fatalf("uptime differ. Got %d Want %d", got, want)
			}
		})
	}

	// uptime is not supported in other OS, so an error should be raised
	getUptime = func() (time.Duration, error) {
		return 0, status.Errorf(codes.Unimplemented, "uptime is not supported")
	}
	t.Cleanup(func() { getUptime = savedUptime })
	for _, tc := range []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "uptime action not suported in other OS except Linux right now",
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
