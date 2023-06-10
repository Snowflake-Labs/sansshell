//go:build linux
// +build linux

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
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"

	pb "github.com/Snowflake-Labs/sansshell/services/power"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
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

func TestReboot(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	c := pb.NewPowerClient(conn)

	savedGenerateReboot := generateReboot

	var cmdLine string
	generateReboot = func(r *pb.RebootRequest) ([]string, error) {
		out, err := savedGenerateReboot(r)
		if err != nil {
			return nil, err
		}

		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "echo"), "-n", "stuff"}, nil
	}
	t.Cleanup(func() { generateReboot = savedGenerateReboot })

	for _, tc := range []struct {
		name string
		want string
		req  *pb.RebootRequest
	}{
		{
			name: "reboot now",
			want: fmt.Sprintf("%s -r now", shutdownBin),
			req: &pb.RebootRequest{
				When: "now",
			},
		},
		{
			name: "reboot now also",
			want: fmt.Sprintf("%s -r +0", shutdownBin),
			req: &pb.RebootRequest{
				When: "+0",
			},
		},
		{
			name: "reboot at 10 am with message",
			want: fmt.Sprintf("%s -r 10:00 badcoffee", shutdownBin),
			req: &pb.RebootRequest{
				When:    "10:00",
				Message: "badcoffee",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := c.Reboot(ctx, tc.req)
			testutil.FatalOnErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			if got, want := cmdLine, tc.want; got != want {
				t.Fatalf("command lines differ. Got %q Want %q", got, want)
			}
		})
	}

	for _, tc := range []struct {
		name string
		req  *pb.RebootRequest
	}{
		{
			name: "error: reboot today",
			req: &pb.RebootRequest{
				When: "today",
			},
		},
		{
			name: "error reboot plus with min",
			req: &pb.RebootRequest{
				When: "+0m",
			},
		},
		{
			name: "error reboot at 10 am",
			req: &pb.RebootRequest{
				When: "10:00am",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := c.Reboot(ctx, tc.req)
			testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			t.Logf("%s: %v", tc.name, err)
		})
	}
}
