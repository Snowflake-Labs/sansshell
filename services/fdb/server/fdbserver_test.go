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
	"strings"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestFDBServer(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewServerClient(conn)

	savedGenerateFDBServerArgs := generateFDBServerArgs
	t.Cleanup(func() {
		generateFDBServerArgs = savedGenerateFDBServerArgs
	})

	bin := testutil.ResolvePath(t, "echo")
	var commands []string

	generateFDBServerArgs = func(req *pb.FDBServerRequest) ([]string, error) {
		commands, err = savedGenerateFDBServerArgs(req)
		return []string{bin, strings.Join(commands, " ")}, err
	}

	for _, tc := range []struct {
		name       string
		req        *pb.FDBServerRequest
		output     *pb.FDBServerResponse
		wantAnyErr bool
		command    []string
	}{
		{
			name:       "missing request",
			req:        &pb.FDBServerRequest{},
			wantAnyErr: true,
		},
		{
			name: "nil command",
			req: &pb.FDBServerRequest{
				Version: nil,
			},
			wantAnyErr: true,
		},
		{
			name: "didn't set version flag, the --version flag should not appear",
			req: &pb.FDBServerRequest{
				Version: &wrapperspb.BoolValue{Value: false},
			},
			output: &pb.FDBServerResponse{
				Stdout: []byte(fmt.Sprintf("%s\n", FDBServer)),
			},
			command: []string{
				FDBServer,
			},
		},
		{
			name: "set version flag",
			req: &pb.FDBServerRequest{
				Version: &wrapperspb.BoolValue{Value: true},
			},
			output: &pb.FDBServerResponse{
				Stdout: []byte(fmt.Sprintf("%s --version\n", FDBServer)),
			},
			command: []string{
				FDBServer,
				"--version",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.FDBServer(ctx, tc.req)
			if tc.wantAnyErr {
				testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
				t.Logf("%s: %v", tc.name, err)
				return
			}
			testutil.FatalOnErr("fdbserver failed", err, t)
			if tc.command != nil {
				testutil.DiffErr("diff commands", commands, tc.command, t)
			}
			if tc.output != nil {
				testutil.DiffErr("output", resp, tc.output, t)
			}

		})
	}

}
