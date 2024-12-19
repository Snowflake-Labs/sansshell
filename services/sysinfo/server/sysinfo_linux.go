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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/euank/go-kmsg-parser/v2/kmsgparser"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// for testing
var (
	journalctlBin = "/usr/bin/journalctl"

	getKmsgParser = func() (kmsgparser.Parser, error) {
		return kmsgparser.NewParser()
	}

	generateJournalCmd = func(p *pb.JournalRequest) ([]string, error) {
		cmd := []string{journalctlBin}

		if p.Unit != "" {
			cmd = append(cmd, fmt.Sprintf("--unit=%s", p.Unit))
		}
		if p.TailLine == 0 {
			return nil, status.Errorf(codes.InvalidArgument, "cannot tail zero journal entry")
		} else if p.TailLine > 0 {
			cmd = append(cmd, fmt.Sprintf("--lines=%d", p.TailLine))
		}
		if p.TimeSince != nil {
			timeStr := p.TimeSince.AsTime().In(time.Local).Format(pb.TimeFormat_YYYYMMDDHHMMSS)
			cmd = append(cmd, fmt.Sprintf("--since=%v", timeStr))
		}
		if p.TimeUntil != nil {
			timeStr := p.TimeUntil.AsTime().In(time.Local).Format(pb.TimeFormat_YYYYMMDDHHMMSS)
			cmd = append(cmd, fmt.Sprintf("--until=%v", timeStr))
		}
		// since json output contains all necessary information we need for now
		// set the format and extract fields we need
		cmd = append(cmd, "--output=json")
		return cmd, nil
	}
)

var getUptime = func() (time.Duration, error) {
	sysinfo := &unix.Sysinfo_t{}
	if err := unix.Sysinfo(sysinfo); err != nil {
		return 0, status.Errorf(codes.Internal, "err in get the system info from unix")
	}
	uptime := time.Duration(sysinfo.Uptime) * time.Second
	return uptime, nil
}

// Based on: https://pkg.go.dev/github.com/euank/go-kmsg-parser
// kmsg-parser only allows us to read message from /dev/kmsg in a blocking way
// we set 2 seconds timeout to explicitly close the channel
// If the package releases new feature to support non-blocking read, we can
// make corresponding changes below to get rid of hard code timeout setting
var getKernelMessages = func(timeout time.Duration, cancelCh <-chan struct{}) ([]*pb.DmsgRecord, error) {
	parser, err := getKmsgParser()
	if err != nil {
		return nil, err
	}

	var records []*pb.DmsgRecord
	messages := parser.Parse()
	done := false
	timeoutCh := time.After(timeout)
	for !done {
		// Select doesn't care about the order of statements, a chatty enough kernel will continue pushing messages
		// into kmsg and therefore our cancellation and timeout logic will not be reached ever,
		// so we do this check first to ensure we don't miss our "deadlines" or client-side cancellation
		select {
		case <-cancelCh:
			parser.Close()
			done = true
			continue
		default:
		}
		select {
		case <-timeoutCh:
			parser.Close()
			done = true
			continue
		default:
		}

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

			// messages channel can have excessive idle time, we want to utilize that to avoid excessive CPU usage
			// hence we do a blocking read of the messages channel (no default statement) but at the same time
			// do blocking read from other channels in case this idle window exceeds timeout or if client cancels command
		case <-timeoutCh:
			parser.Close()
			done = true
		case <-cancelCh:
			parser.Close()
			done = true
		}
	}
	return records, nil
}

var getJournalRecordsAndSend = func(ctx context.Context, req *pb.JournalRequest, stream pb.SysInfo_JournalServer) error {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	command, err := generateJournalCmd(req)
	if err != nil {
		recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
		return err
	}
	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "run_err"))
		return err
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "run_err"))
		return status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	// parse the output
	scanner := bufio.NewScanner(run.Stdout)

	for scanner.Scan() {
		// Based on https://man.archlinux.org/man/journalctl.1#OUTPUT_OPTIONS
		// set output to json will "format entries as JSON objects, separated by newline characters"
		// so we can parse each entry line by line
		text := scanner.Text()

		// Create a map to hold the parsed data
		var journalMap map[string]string

		// Parse the JSON string
		err := json.Unmarshal([]byte(text), &journalMap)
		if err != nil {
			return status.Errorf(codes.Internal, "parse the journal entry from json string to map err: %v", err)
		}
		// EnableJson: true means return
		if req.EnableJson {
			journalRecordRaw := &pb.JournalRecordRaw{}
			journalRecordRaw.Entry = journalMap
			if err := stream.Send(&pb.JournalReply{
				Response: &pb.JournalReply_JournalRaw{
					JournalRaw: journalRecordRaw,
				},
			}); err != nil {
				recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "stream_send_err"))
				return status.Errorf(codes.Internal, "journal: send error %v", err)
			}
		} else {
			// default format
			journalRecord := &pb.JournalRecord{}

			// Parse the string value as an int64
			realtime, err := strconv.ParseInt(journalMap["__REALTIME_TIMESTAMP"], 10, 64)
			if err != nil {
				return status.Errorf(codes.Internal, "journal entry realtime converts error: %v from string to int64", err)
			}
			journalRecord.RealtimeTimestamp = timestamppb.New(time.Unix(0, realtime*int64(time.Microsecond)))
			journalRecord.Hostname = journalMap["_HOSTNAME"]
			journalRecord.SyslogIdentifier = journalMap["SYSLOG_IDENTIFIER"]
			journalRecord.Message = journalMap["MESSAGE"]

			// some log entries may not have pid, since they are not generated by a process
			if pidStr, ok := journalMap["_PID"]; ok {
				pid, err := strconv.Atoi(pidStr)
				if err != nil {
					return status.Errorf(codes.Internal, "pid converts error: %v from string to int32", err)
				}
				journalRecord.Pid = int32(pid)
			}
			// send the record directly
			if err := stream.Send(&pb.JournalReply{
				Response: &pb.JournalReply_Journal{
					Journal: journalRecord,
				},
			}); err != nil {
				recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "stream_send_err"))
				return status.Errorf(codes.Internal, "journal: send error %v", err)
			}
		}
	}
	return nil
}
