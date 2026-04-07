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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
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
	getKernelMessages = func(time.Duration, <-chan struct{}) ([]*pb.DmsgRecord, error) {
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

	getKernelMessages = func(time.Duration, <-chan struct{}) ([]*pb.DmsgRecord, error) {
		_, err := savedGetKernelMessages(time.Duration(0), nil)
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

// readJournalLog reads the test fixture and returns:
// 1. array of raw JSON lines (for feeding to echo in mocked commands)
// 2. array of map[string]string parsed through the same pipeline as production code
// 3. error
func readJournalLog() ([]string, []map[string]string, error) {
	return readJournalLogFile("./testdata/journal-log-entries.txt")
}

func readJournalLogFile(path string) ([]string, []map[string]string, error) {
	var result []map[string]string
	input, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(string(input), "\n")
	var nonEmptyLines []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		nonEmptyLines = append(nonEmptyLines, line)
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, nil, err
		}
		result = append(result, journalEntryToStringMap(raw))
	}
	return nonEmptyLines, result, nil
}

// construct the expected JournalReply based on the map given
// wantRaw means whether we want th return back the map directly, usally happens with json or json-pretty output
func getJournalReply(raw map[string]string, wantRaw bool, t *testing.T) *pb.JournalReply {
	if wantRaw {
		return &pb.JournalReply{
			Response: &pb.JournalReply_JournalRaw{
				JournalRaw: &pb.JournalRecordRaw{
					Entry: raw,
				},
			},
		}
	}
	timestampInt, err := strconv.ParseInt(raw["__REALTIME_TIMESTAMP"], 10, 64)
	testutil.FatalOnErr("Failed to convert realtimestamp from string to int64", err, t)
	realtime := timestamppb.New(time.Unix(0, timestampInt*int64(time.Microsecond)))
	pidInt, err := strconv.ParseInt(raw["_PID"], 10, 32)
	testutil.FatalOnErr("Failed to convert pid from string to int32", err, t)
	reply := &pb.JournalRecord{
		RealtimeTimestamp: realtime,
		Hostname:          raw["_HOSTNAME"],
		SyslogIdentifier:  raw["SYSLOG_IDENTIFIER"],
		Pid:               int32(pidInt),
		Message:           raw["MESSAGE"],
	}

	return &pb.JournalReply{
		Response: &pb.JournalReply_Journal{
			Journal: reply,
		},
	}
}

func TestJournal(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewSysInfoClient(conn)

	// prepare the data
	rawDataList, rawDataMapList, err := readJournalLog()
	testutil.FatalOnErr("Failed to read raw journal data", err, t)
	// process from input time to timestamppb
	startTime := "2023-07-27 17:19:00"
	endTime := "2023-07-27 17:20:00"
	expectedTimeFormat := "2006-01-02 15:04:05"
	loc, err := time.LoadLocation("Local")
	testutil.FatalOnErr("Failed to load local location", err, t)
	sinceTime, err := time.ParseInLocation(expectedTimeFormat, startTime, loc)
	testutil.FatalOnErr("Failed to convert since time in specified time zone", err, t)
	untilTime, err := time.ParseInLocation(expectedTimeFormat, endTime, loc)
	testutil.FatalOnErr("Failed to convert until time in specified time zone", err, t)
	sinceTimeTimestamp := timestamppb.New(sinceTime)
	untiltimeTimestamp := timestamppb.New(untilTime)

	journalctlBin = "journalctl"

	savedGenerateJournalCmd := generateJournalCmd

	// test in Linux env
	var cmdLine string

	for _, tc := range []struct {
		name          string
		req           *pb.JournalRequest
		testdataInput []string
		isOutputJson  bool
		wantCmdLine   string
		wantReply     []*pb.JournalReply
		wantErr       bool
	}{
		{
			name: "fetch all journal entries: journalctl -tail 100",
			req: &pb.JournalRequest{
				TailLine: 100,
			},
			testdataInput: rawDataList,
			wantCmdLine:   fmt.Sprintf("%s --lines=100 --output=json", journalctlBin),
			wantReply: []*pb.JournalReply{
				getJournalReply(rawDataMapList[0], false, t),
				getJournalReply(rawDataMapList[1], false, t),
				getJournalReply(rawDataMapList[2], false, t),
				getJournalReply(rawDataMapList[3], false, t),
			},
		},
		{
			name: "fetch the latest one journal entry: journalctl -tail 1",
			req: &pb.JournalRequest{
				TailLine: 1,
			},
			testdataInput: []string{rawDataList[3]},
			wantCmdLine:   fmt.Sprintf("%s --lines=1 --output=json", journalctlBin),
			wantReply: []*pb.JournalReply{
				getJournalReply(rawDataMapList[3], false, t),
			},
		},
		{
			name: "filter journal entries with systemd unit boot.mount and augument text: journalctl -x -unit boot.mount",
			req: &pb.JournalRequest{
				TailLine: 100,
				Unit:     "boot.mount",
			},
			testdataInput: []string{rawDataList[0], rawDataList[1]},
			wantCmdLine:   fmt.Sprintf("%s --unit=boot.mount --lines=100 --output=json", journalctlBin),
			wantReply: []*pb.JournalReply{
				getJournalReply(rawDataMapList[0], false, t),
				getJournalReply(rawDataMapList[1], false, t),
			},
		},
		{
			name: "filter journal entries based on time : journalctl --since '2023-07-27 17:19:00' --until '2023-07-27 17:20:00'",
			req: &pb.JournalRequest{
				TailLine:  100,
				TimeSince: sinceTimeTimestamp,
				TimeUntil: untiltimeTimestamp,
			},
			testdataInput: []string{rawDataList[2]},
			wantCmdLine:   fmt.Sprintf("%s --lines=100 --since=%s --until=%s --output=json", journalctlBin, startTime, endTime),
			wantReply: []*pb.JournalReply{
				getJournalReply(rawDataMapList[2], false, t),
			},
		},
		{
			name: "fetch latest entry with json format: journalctl -tail 1 -json",
			req: &pb.JournalRequest{
				TailLine:   1,
				EnableJson: true,
			},
			testdataInput: []string{rawDataList[3]},
			wantCmdLine:   fmt.Sprintf("%s --lines=1 --output=json", journalctlBin),
			wantReply: []*pb.JournalReply{
				getJournalReply(rawDataMapList[3], true, t),
			},
		},
		{
			name: "bad command large tail number : journalctl -tail 10003 ",
			req: &pb.JournalRequest{
				TailLine: 10003,
			},
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			generateJournalCmd = func(p *pb.JournalRequest) ([]string, error) {
				command, err := savedGenerateJournalCmd(p)
				if err != nil {
					return nil, err
				}
				cmdLine = strings.Join(command, " ")
				return []string{testutil.ResolvePath(t, "echo"), "-n", strings.Join(tc.testdataInput, "\n")}, nil
			}
			stream, err := client.Journal(ctx, tc.req)
			testutil.FatalOnErr("Journal failed", err, t)
			var gotRecords []*pb.JournalReply
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
				gotRecords = append(gotRecords, resp)
			}

			// no matter the reply is JournalReply_Journal or JournalReply_JournalRaw, just compare them
			testutil.DiffErr(tc.name, gotRecords, tc.wantReply, t)
			// test command line
			if got, want := cmdLine, tc.wantCmdLine; got != want {
				t.Fatalf("command lines differ for journal action. Got %q Want %q", got, want)
			}
		})
	}

	// journal is not supported in other OS, so an error should be raised
	savedGetJournalRecordsAndSend := getJournalRecordsAndSend
	getJournalRecordsAndSend = func(ctx context.Context, req *pb.JournalRequest, stream pb.SysInfo_JournalServer) error {
		return status.Errorf(codes.Unimplemented, "journal is not supported")
	}
	t.Cleanup(func() {
		generateJournalCmd = savedGenerateJournalCmd
		getJournalRecordsAndSend = savedGetJournalRecordsAndSend
	})
	for _, tc := range []struct {
		name    string
		req     *pb.JournalRequest
		wantErr bool
	}{
		{
			name:    "journal action not supported in other OS except Linux right now",
			req:     &pb.JournalRequest{},
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stream, _ := client.Journal(ctx, tc.req)
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
}

func TestSanitizeString(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "printable ASCII unchanged",
			in:   "Hello, World!",
			want: "Hello, World!",
		},
		{
			name: "whitespace preserved",
			in:   "line1\nline2\ttab\rcarriage",
			want: "line1\nline2\ttab\rcarriage",
		},
		{
			name: "null byte replaced",
			in:   "a\x00b",
			want: "a\uFFFDb",
		},
		{
			name: "control characters replaced",
			in:   "a\x01\x02\x03\x1fb",
			want: "a\uFFFD\uFFFD\uFFFD\uFFFDb",
		},
		{
			name: "valid UTF-8 multibyte preserved",
			in:   "caf\u00e9 \u2603",
			want: "caf\u00e9 \u2603",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "high bytes forming invalid UTF-8 replaced",
			in:   string([]byte{0x80, 0x81, 0xFE, 0xFF}),
			want: "\uFFFD\uFFFD\uFFFD\uFFFD",
		},
		{
			name: "mixed valid and invalid bytes",
			in:   "ok" + string([]byte{0x80}) + "ok",
			want: "ok\uFFFDok",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeString(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeString(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestJournalValueToString(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want string
	}{
		{
			name: "plain string passes through unchanged",
			in:   "hello world",
			want: "hello world",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "printable byte array becomes string",
			in:   []any{float64(72), float64(105)},
			want: "Hi",
		},
		{
			name: "byte array with null byte gets sanitized",
			in:   []any{float64(72), float64(101), float64(108), float64(108), float64(111), float64(0), float64(87), float64(111), float64(114), float64(108), float64(100)},
			want: "Hello\uFFFDWorld",
		},
		{
			name: "byte array with multiple non-printable bytes",
			in:   []any{float64(65), float64(1), float64(2), float64(66)},
			want: "A\uFFFD\uFFFDB",
		},
		{
			name: "empty byte array",
			in:   []any{},
			want: "",
		},
		{
			name: "single byte array element",
			in:   []any{float64(65)},
			want: "A",
		},
		{
			name: "high bytes 128-255 get sanitized as invalid UTF-8",
			in:   []any{float64(0x80), float64(0xFE), float64(0xFF)},
			want: "\uFFFD\uFFFD\uFFFD",
		},
		{
			name: "all non-printable byte array",
			in:   []any{float64(0), float64(1), float64(2), float64(3)},
			want: "\uFFFD\uFFFD\uFFFD\uFFFD",
		},
		{
			name: "byte array preserving tabs and newlines",
			in:   []any{float64(65), float64(9), float64(10), float64(13), float64(66)},
			want: "A\t\n\rB",
		},
		{
			name: "byte array with valid UTF-8 multibyte sequence",
			in:   []any{float64(0xC3), float64(0xA9)},
			want: "\u00e9",
		},
		{
			name: "boundary value 255",
			in:   []any{float64(255)},
			want: "\uFFFD",
		},
		{
			name: "boundary value 0",
			in:   []any{float64(0)},
			want: "\uFFFD",
		},
		{
			name: "array with non-numeric element falls back to Sprintf",
			in:   []any{"not", "bytes"},
			want: "[not bytes]",
		},
		{
			name: "array with mixed types falls back to Sprintf",
			in:   []any{float64(72), "mixed"},
			want: "[72 mixed]",
		},
		{
			name: "numeric non-integer value falls back to Sprintf",
			in:   []any{float64(72.5)},
			want: "[72.5]",
		},
		{
			name: "number out of byte range falls back to Sprintf",
			in:   []any{float64(256)},
			want: "[256]",
		},
		{
			name: "negative number falls back to Sprintf",
			in:   []any{float64(-1)},
			want: "[-1]",
		},
		{
			name: "float64 number becomes its string representation",
			in:   float64(42),
			want: "42",
		},
		{
			name: "bool becomes its string representation",
			in:   true,
			want: "true",
		},
		{
			name: "nil becomes <nil>",
			in:   nil,
			want: "<nil>",
		},
		{
			name: "nested map falls back to Sprintf",
			in:   map[string]any{"nested": "value"},
			want: "map[nested:value]",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := journalValueToString(tc.in)
			if got != tc.want {
				t.Errorf("journalValueToString(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestJournalEntryToStringMap(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   map[string]any
		want map[string]string
	}{
		{
			name: "mixed string and byte-array values",
			in: map[string]any{
				"MESSAGE":              []any{float64(72), float64(101), float64(108), float64(108), float64(111), float64(0)},
				"PRIORITY":             "6",
				"SYSLOG_IDENTIFIER":    "myservice",
				"__REALTIME_TIMESTAMP": "1687391641745630",
			},
			want: map[string]string{
				"MESSAGE":              "Hello\uFFFD",
				"PRIORITY":             "6",
				"SYSLOG_IDENTIFIER":    "myservice",
				"__REALTIME_TIMESTAMP": "1687391641745630",
			},
		},
		{
			name: "empty map",
			in:   map[string]any{},
			want: map[string]string{},
		},
		{
			name: "multiple byte-array fields",
			in: map[string]any{
				"MESSAGE":  []any{float64(65), float64(0), float64(66)},
				"FIELD_X":  []any{float64(67), float64(68)},
				"PRIORITY": "3",
			},
			want: map[string]string{
				"MESSAGE":  "A\uFFFDB",
				"FIELD_X":  "CD",
				"PRIORITY": "3",
			},
		},
		{
			name: "all string values unchanged",
			in: map[string]any{
				"MESSAGE":  "normal",
				"PRIORITY": "6",
			},
			want: map[string]string{
				"MESSAGE":  "normal",
				"PRIORITY": "6",
			},
		},
		{
			name: "JSON number value stringified",
			in: map[string]any{
				"MESSAGE":  "test",
				"PRIORITY": float64(6),
			},
			want: map[string]string{
				"MESSAGE":  "test",
				"PRIORITY": "6",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := journalEntryToStringMap(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("map length = %d, want %d", len(got), len(tc.want))
			}
			for k, wantV := range tc.want {
				if gotV, ok := got[k]; !ok {
					t.Errorf("missing key %q", k)
				} else if gotV != wantV {
					t.Errorf("key %q = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

// TestJournalByteArrayRegression is the regression test for the original bug:
// json.Unmarshal into map[string]string fails when journalctl emits MESSAGE
// as an array of byte values. This proves the old code would break and the
// new code handles it.
func TestJournalByteArrayRegression(t *testing.T) {
	jsonWithByteArray := `{"MESSAGE":[72,101,108,108,111,0,87,111,114,108,100],"PRIORITY":"6","_PID":"1","_HOSTNAME":"host","SYSLOG_IDENTIFIER":"myservice","__REALTIME_TIMESTAMP":"1687391641745630"}`

	// The old approach: unmarshal into map[string]string -- must fail
	var oldMap map[string]string
	err := json.Unmarshal([]byte(jsonWithByteArray), &oldMap)
	if err == nil {
		t.Fatal("expected json.Unmarshal into map[string]string to fail for byte-array MESSAGE, but it succeeded")
	}

	// The new approach: unmarshal into map[string]any then convert -- must succeed
	var newMap map[string]any
	err = json.Unmarshal([]byte(jsonWithByteArray), &newMap)
	if err != nil {
		t.Fatalf("json.Unmarshal into map[string]any failed: %v", err)
	}
	result := journalEntryToStringMap(newMap)

	if result["MESSAGE"] != "Hello\uFFFDWorld" {
		t.Errorf("MESSAGE = %q, want %q", result["MESSAGE"], "Hello\uFFFDWorld")
	}
	if result["PRIORITY"] != "6" {
		t.Errorf("PRIORITY = %q, want %q", result["PRIORITY"], "6")
	}
	if result["_PID"] != "1" {
		t.Errorf("_PID = %q, want %q", result["_PID"], "1")
	}
	if result["__REALTIME_TIMESTAMP"] != "1687391641745630" {
		t.Errorf("__REALTIME_TIMESTAMP = %q, want %q", result["__REALTIME_TIMESTAMP"], "1687391641745630")
	}
}

func TestJournalByteArrayMessage(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewSysInfoClient(conn)

	rawDataList, rawDataMapList, err := readJournalLogFile("./testdata/journal-log-entries-bytearray.txt")
	testutil.FatalOnErr("Failed to read byte-array journal data", err, t)

	journalctlBin = "journalctl"
	savedGenerateJournalCmd := generateJournalCmd
	t.Cleanup(func() { generateJournalCmd = savedGenerateJournalCmd })

	for _, tc := range []struct {
		name          string
		req           *pb.JournalRequest
		testdataInput []string
		wantReply     []*pb.JournalReply
	}{
		{
			name: "byte-array MESSAGE parsed correctly in raw JSON mode",
			req: &pb.JournalRequest{
				TailLine:   100,
				EnableJson: true,
			},
			testdataInput: rawDataList,
			wantReply: func() []*pb.JournalReply {
				var replies []*pb.JournalReply
				for _, m := range rawDataMapList {
					replies = append(replies, getJournalReply(m, true, t))
				}
				return replies
			}(),
		},
		{
			name: "byte-array MESSAGE parsed correctly in structured mode",
			req: &pb.JournalRequest{
				TailLine: 100,
			},
			testdataInput: rawDataList,
			wantReply: func() []*pb.JournalReply {
				var replies []*pb.JournalReply
				for _, m := range rawDataMapList {
					replies = append(replies, getJournalReply(m, false, t))
				}
				return replies
			}(),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			generateJournalCmd = func(p *pb.JournalRequest) ([]string, error) {
				command, err := savedGenerateJournalCmd(p)
				if err != nil {
					return nil, err
				}
				_ = command
				return []string{testutil.ResolvePath(t, "echo"), "-n", strings.Join(tc.testdataInput, "\n")}, nil
			}
			stream, err := client.Journal(ctx, tc.req)
			testutil.FatalOnErr("Journal failed", err, t)
			var gotRecords []*pb.JournalReply
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				testutil.FatalOnErr("unexpected error receiving journal reply", err, t)
				gotRecords = append(gotRecords, resp)
			}
			testutil.DiffErr(tc.name, gotRecords, tc.wantReply, t)
		})
	}
}
