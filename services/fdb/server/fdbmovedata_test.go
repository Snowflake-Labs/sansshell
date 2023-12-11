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
	"io"
	"time"

	"strings"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func TestFDBMoveData(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewFDBMoveClient(conn)

	savedGenerateFDBMoveDataArgs := generateFDBMoveDataArgs
	t.Cleanup(func() {
		generateFDBMoveDataArgs = savedGenerateFDBMoveDataArgs
	})

	bin := testutil.ResolvePath(t, "echo")
	var commands []string

	generateFDBMoveDataArgs = func(req *pb.FDBMoveDataCopyRequest) ([]string, error) {
		commands, err = savedGenerateFDBMoveDataArgs(req)
		return []string{bin, strings.Join(commands, " ")}, err
	}

	for _, tc := range []struct {
		name       string
		reqCopy    *pb.FDBMoveDataCopyRequest
		outputWait []*pb.FDBMoveDataWaitResponse
	}{
		{
			name: "proper",
			reqCopy: &pb.FDBMoveDataCopyRequest{
				ClusterFile:        "1",
				TenantGroup:        "2",
				SourceCluster:      "3",
				DestinationCluster: "4",
				NumProcs:           5,
			},
			outputWait: []*pb.FDBMoveDataWaitResponse{
				{
					Stdout: []byte(" --cluster 1 --tenant-group 2 --src-name 3 --dst-name 4 --num-procs 5\n"),
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.FDBMoveDataCopy(ctx, tc.reqCopy)
			testutil.FatalOnErr("fdbmovedata copy failed", err, t)
			waitReq := &pb.FDBMoveDataWaitRequest{
				Id: resp.Id,
			}
			waitResp, err := client.FDBMoveDataWait(ctx, waitReq)
			testutil.FatalOnErr("fdbmovedata wait failed", err, t)
			for _, want := range tc.outputWait {
				rs, err := waitResp.Recv()
				if err != nil {
					testutil.FatalOnErr("fdbmovedata wait recv failed", err, t)
				}
				if !(proto.Equal(want, rs)) {
					t.Errorf("want: %v, got: %v", want, rs)
				}
			}
			_, err = waitResp.Recv()
			if err != io.EOF {
				t.Error("unexpected")
			}
		})
	}

}

func TestFDBMoveDataDouble(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	client1 := pb.NewFDBMoveClient(conn)
	client2 := pb.NewFDBMoveClient(conn)

	savedGenerateFDBMoveDataArgs := generateFDBMoveDataArgs
	t.Cleanup(func() {
		generateFDBMoveDataArgs = savedGenerateFDBMoveDataArgs
	})

	sh := testutil.ResolvePath(t, "/bin/sh")

	generateFDBMoveDataArgs = func(req *pb.FDBMoveDataCopyRequest) ([]string, error) {
		_, err = savedGenerateFDBMoveDataArgs(req)
		return []string{sh, "-c", "/bin/sleep 1; echo done"}, err
	}
	for _, tc := range []struct {
		name       string
		reqCopy    *pb.FDBMoveDataCopyRequest
		outputWait []*pb.FDBMoveDataWaitResponse
	}{
		{
			name: "double",
			reqCopy: &pb.FDBMoveDataCopyRequest{
				ClusterFile:        "1",
				TenantGroup:        "2",
				SourceCluster:      "3",
				DestinationCluster: "4",
				NumProcs:           5,
			},
			outputWait: []*pb.FDBMoveDataWaitResponse{
				{
					Stdout: []byte("done\n"),
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp1, errCopy1 := client1.FDBMoveDataCopy(ctx, tc.reqCopy)
			resp2, errCopy2 := client2.FDBMoveDataCopy(ctx, tc.reqCopy)
			testutil.FatalOnErr("fdbmovedata copy1 failed", errCopy1, t)
			testutil.FatalOnErr("fdbmovedata copy2 failed", errCopy2, t)

			if !(resp1.Id == resp2.Id) {
				t.Errorf("want: %v, got: %v", resp1.Id, resp2.Id)
			}
			if !(resp2.Existing) {
				t.Errorf("Expected resp2 to return an existing run")
			}

			waitReq := &pb.FDBMoveDataWaitRequest{
				Id: resp1.Id,
			}
			waitResp1, err1 := client1.FDBMoveDataWait(ctx, waitReq)
			testutil.FatalOnErr("fdbmovedata wait1 failed", err1, t)
			for _, want1 := range tc.outputWait {
				rs, err1 := waitResp1.Recv()
				if err1 != io.EOF {
					testutil.FatalOnErr("fdbmovedata wait1 failed", err1, t)
				}
				if !(proto.Equal(want1, rs)) {
					t.Errorf("want: %v, got: %v", want1, rs)
				}
			}
			waitResp2, err2 := client2.FDBMoveDataWait(ctx, waitReq)
			testutil.FatalOnErr("fdbmovedata wait2 failed", err2, t)

			rs, err2 := waitResp2.Recv()
			if err2 != io.EOF {
				testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, rs), err2, t)
			}
		})
	}
}

func TestFDBMoveDataResumed(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewFDBMoveClient(conn)

	savedGenerateFDBMoveDataArgs := generateFDBMoveDataArgs
	t.Cleanup(func() {
		generateFDBMoveDataArgs = savedGenerateFDBMoveDataArgs
	})

	sh := testutil.ResolvePath(t, "/bin/sh")

	generateFDBMoveDataArgs = func(req *pb.FDBMoveDataCopyRequest) ([]string, error) {
		_, err = savedGenerateFDBMoveDataArgs(req)
		return []string{sh, "-c", "/bin/sleep 1; for i in {1..16000}; do echo filling-up-pipe-buffer; done"}, err
	}

	resp, err := client.FDBMoveDataCopy(ctx, &pb.FDBMoveDataCopyRequest{
		ClusterFile:        "1",
		TenantGroup:        "2",
		SourceCluster:      "3",
		DestinationCluster: "4",
		NumProcs:           5,
	})
	testutil.FatalOnErr("fdbmovedata copy failed", err, t)
	waitReq := &pb.FDBMoveDataWaitRequest{Id: resp.Id}
	shortCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	shortWaitResp, err := client.FDBMoveDataWait(shortCtx, waitReq)
	testutil.FatalOnErr("fdbmovedata wait failed", err, t)
	_, err = shortWaitResp.Recv()
	testutil.WantErr("fdbmovedata wait", err, true, t)

	time.Sleep(20 * time.Millisecond)

	waitResp, err := client.FDBMoveDataWait(ctx, waitReq)
	testutil.FatalOnErr("second fdbmovedata wait failed", err, t)
	for {
		if _, err := waitResp.Recv(); err != nil {
			if err != io.EOF {
				testutil.FatalOnErr("fdbmovedata wait exited with error", err, t)
			}
			break
		}
	}
}
