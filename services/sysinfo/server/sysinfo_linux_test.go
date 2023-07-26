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
	"io"
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
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/euank/go-kmsg-parser/v2/kmsgparser"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
	conn    *grpc.ClientConn
)

// mock KmsgParser for testing: some testing environment doesn't allow
// KmsgParser read file from /dev/kmsg
type mockKmsgParser struct{}

func (m *mockKmsgParser) Parse() <-chan kmsgparser.Message {
	output := make(chan kmsgparser.Message, 1)
	message := kmsgparser.Message{}
	output <- message
	return output
}

func (m *mockKmsgParser) Close() error {
	return nil
}

func (m *mockKmsgParser) SeekEnd() error {
	return nil
}

func (m *mockKmsgParser) SetLogger(kmsgparser.Logger) {}

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

func TestDmesg(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewSysInfoClient(conn)

	// Mock the getKmsgParser function to return a mockKmsgParser object
	getKmsgParser = func() (kmsgparser.Parser, error) {
		return &mockKmsgParser{}, nil
	}

	// Message mocks the kmsgparser.Message it process from /dev/kmsg
	type Message struct {
		message   string
		timestamp time.Time
	}
	timestamp := time.Now()
	wantTime := timestamppb.New(timestamp)
	rawRecords := [3]Message{
		{
			message:   "xfs filesystem being remounted at /tmp supports timestamps until 2038 (0x7fffffff)",
			timestamp: timestamp,
		},
		{
			message: `usb 1-1: SerialNumber: TAG11d87aca0
			SUBSYSTEM=usb
			DEVICE=c189:3`,
			timestamp: timestamp,
		},
		{
			message:   "CPU features: detected: Address authentication (IMP DEF algorithm)",
			timestamp: timestamp,
		},
	}

	savedGetKernelMessages := getKernelMessages

	// dmesg is not supported in other OS, so an error should be raised
	getKernelMessages = func() ([]*pb.DmsgRecord, error) {
		return nil, status.Errorf(codes.Unimplemented, "dmesg is not supported")
	}
	t.Cleanup(func() { getKernelMessages = savedGetKernelMessages })
	for _, tc := range []struct {
		name    string
		req     *pb.DmesgRequest
		wantErr bool
	}{
		{
			name: "dmesg action not supported in other OS except Linux right now",
			req: &pb.DmesgRequest{
				TailLines: -1,
			},
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stream, _ := client.Dmesg(ctx, tc.req)
			for {
				_, err := stream.Recv()
				if err == io.EOF {
					break
				}
				testutil.WantErr(tc.name, err, tc.wantErr, t)
				if tc.wantErr {
					// If this was an expected error we're done.
					return
				}

			}
		})
	}

	getKernelMessages = func() ([]*pb.DmsgRecord, error) {
		_, err := savedGetKernelMessages()
		if err != nil {
			return nil, err
		}
		var records []*pb.DmsgRecord
		for _, raw := range rawRecords {
			records = append(records, &pb.DmsgRecord{
				Message: raw.message,
				Time:    timestamppb.New(raw.timestamp),
			})
		}
		return records, nil
	}

	for _, tc := range []struct {
		name      string
		req       *pb.DmesgRequest
		wantReply []*pb.DmesgReply
		wantErr   bool
	}{
		{
			name: "specify ignore case without provide grep expect an error: dmesg",
			req: &pb.DmesgRequest{
				IgnoreCase: true,
			},
			wantErr: true,
		},
		{
			name: "specify invert match without provide grep expect an error: dmesg",
			req: &pb.DmesgRequest{
				InvertMatch: true,
			},
			wantErr: true,
		},
		{
			name: "fetch all kernel messages",
			req: &pb.DmesgRequest{
				TailLines: -1,
			},
			wantReply: []*pb.DmesgReply{
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[0].message,
					},
				},
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[1].message,
					},
				},
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[2].message,
					},
				},
			},
		},
		{
			name: "tailLines larger than initial records will still fetch all kernel messagesL dmesg -tail 100",
			req: &pb.DmesgRequest{
				TailLines: 100,
			},
			wantReply: []*pb.DmesgReply{
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[0].message,
					},
				},
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[1].message,
					},
				},
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[2].message,
					},
				},
			},
		},
		{
			name: "fetch last one kernel message: dmesg -tail 1",
			req: &pb.DmesgRequest{
				TailLines: 1,
			},
			wantReply: []*pb.DmesgReply{
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[2].message,
					},
				},
			},
		},
		{
			name: "match string usb with ignore case flag: dmesg -grep Usb -i",
			req: &pb.DmesgRequest{
				TailLines:  -1,
				Grep:       "Usb",
				IgnoreCase: true,
			},
			wantReply: []*pb.DmesgReply{
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[1].message,
					},
				},
			},
		},
		{
			name: "match string usb with ignore case and invert match flag: dmesg -grep Usb -i -v",
			req: &pb.DmesgRequest{
				TailLines:   -1,
				Grep:        "Usb",
				IgnoreCase:  true,
				InvertMatch: true,
			},
			wantReply: []*pb.DmesgReply{
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[0].message,
					},
				},
				{
					Record: &pb.DmsgRecord{
						Time:    wantTime,
						Message: rawRecords[2].message,
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stream, err := client.Dmesg(ctx, tc.req)
			testutil.FatalOnErr("Dmesg failed", err, t)
			var gotRecords []*pb.DmsgRecord
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				testutil.WantErr(tc.name, err, tc.wantErr, t)
				if tc.wantErr {
					// If this was an expected error we're done.
					return
				}
				gotRecords = append(gotRecords, resp.Record)

			}
			if wantLen, gotLen := len(tc.wantReply), len(gotRecords); wantLen != gotLen {
				t.Fatalf("dmesg length differ. Got %d Want %d", gotLen, wantLen)
			}
			for idx, record := range gotRecords {
				wantRecord := tc.wantReply[idx].Record
				testutil.DiffErr(tc.name, record, wantRecord, t)
			}
		})
	}
}
