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
	"io"
	"log"
	"net"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/Snowflake-Labs/sansshell/services/fdbexec"
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

func collect(c pb.FdbExec_StreamingRunClient) (*pb.FdbExecResponse, error) {
	collected := &pb.FdbExecResponse{}
	for {
		resp, err := c.Recv()
		if err == io.EOF {
			return collected, nil
		}
		if err != nil {
			return nil, err
		}
		collected.Stdout = append(collected.Stdout, resp.Stdout...)
		collected.Stderr = append(collected.Stderr, resp.Stderr...)
		collected.RetCode = resp.RetCode
	}
}

func TestFdbExec(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewFdbExecClient(conn)

	for _, tc := range []struct {
		name              string
		bin               string
		args              []string
		user              string
		wantErr           bool
		returnCodeNonZero bool
		stdout            string
	}{
		{
			name:   "Basic functionality",
			bin:    testutil.ResolvePath(t, "echo"),
			args:   []string{"hello world"},
			stdout: "hello world\n",
		},
		{
			name:              "Command fails",
			bin:               testutil.ResolvePath(t, "false"),
			returnCodeNonZero: true,
		},
		{
			name:    "Non-existant program",
			bin:     "/something/non-existant",
			wantErr: true,
		},
		{
			name:    "non-absolute path",
			bin:     "foo",
			wantErr: true,
		},
		{
			name:    "user specified -- fails as it can't setuid",
			bin:     testutil.ResolvePath(t, "echo"),
			args:    []string{"hello world"},
			user:    "nobody",
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Test a normal exec.
			resp, err := client.Run(ctx, &pb.FdbExecRequest{
				Command: tc.bin,
				Args:    tc.args,
				User:    tc.user,
			})
			t.Logf("%s: resp: %+v", tc.name, resp)
			t.Logf("%s: err: %v", tc.name, err)
			if tc.wantErr {
				testutil.WantErr(tc.name, err, tc.wantErr, t)
				return
			}
			if got, want := resp.Stdout, tc.stdout; string(got) != want {
				t.Fatalf("%s: stdout doesn't match. Want %q Got %q", tc.name, want, got)
			}
			if got, want := resp.RetCode != 0, tc.returnCodeNonZero; got != want {
				t.Fatalf("%s: Invalid return codes. Non-zero state doesn't match. Want %t Got %t ReturnCode %d", tc.name, want, got, resp.RetCode)
			}

			// Test a streaming exec, which should behave identically to normal exec if
			// all responses are concatenated together.
			stream, err := client.StreamingRun(ctx, &pb.FdbExecRequest{
				Command: tc.bin,
				Args:    tc.args,
			})
			if err != nil {
				t.Fatal(err)
			}
			streamResp, err := collect(stream)
			t.Logf("%s: resp: %+v", tc.name, streamResp)
			t.Logf("%s: err: %v", tc.name, err)
			if tc.wantErr {
				testutil.WantErr(tc.name, err, tc.wantErr, t)
				return
			}
			if got, want := streamResp.Stdout, tc.stdout; string(got) != want {
				t.Fatalf("%s: stdout doesn't match. Want %q Got %q", tc.name, want, got)
			}
			if got, want := streamResp.RetCode != 0, tc.returnCodeNonZero; got != want {
				t.Fatalf("%s: Invalid return codes. Non-zero state doesn't match. Want %t Got %t ReturnCode %d", tc.name, want, got, streamResp.RetCode)
			}

		})
	}
}
