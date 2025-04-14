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

package test

import (
	"context"
	"log"
	"net"
	"os"
	"testing"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	_ "github.com/Snowflake-Labs/sansshell/services/fdb/server" // Initialize the server to register services
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	for _, svc := range services.ListServices() {
		svc.Register(s)
	}
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

// TestFDBBackupCommands tests the basic functionality of fdbbackup commands
func TestFDBBackupCommands(t *testing.T) {
	ctx := context.Background()

	// Create a test connection
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Get a client
	client := pb.NewBackupClient(conn)

	// Test cases
	testCases := []struct {
		name    string
		request *pb.FDBBackupRequest
		wantErr bool
	}{
		{
			name: "status",
			request: &pb.FDBBackupRequest{
				Commands: []*pb.FDBBackupCommand{
					{
						Command: &pb.FDBBackupCommand_Status{
							Status: &pb.FDBBackupStatus{},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "start backup",
			request: &pb.FDBBackupRequest{
				Commands: []*pb.FDBBackupCommand{
					{
						Command: &pb.FDBBackupCommand_Start{
							Start: &pb.FDBBackupStart{
								DestinationUrl: "s3://test-bucket",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "abort backup",
			request: &pb.FDBBackupRequest{
				Commands: []*pb.FDBBackupCommand{
					{
						Command: &pb.FDBBackupCommand_Abort{
							Abort: &pb.FDBBackupAbort{},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.FDBBackup(ctx, tc.request)
			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got success", tc.name)
				}
				return
			}
			testutil.FatalOnErr(tc.name, err, t)
		})
	}
}
