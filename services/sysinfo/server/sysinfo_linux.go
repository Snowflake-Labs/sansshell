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
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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

// sanitizeString replaces non-printable characters (except common whitespace)
// with the Unicode replacement character, ensuring the output is valid UTF-8.
func sanitizeString(s string) string {
	return strings.Map(func(r rune) rune {
		if r == utf8.RuneError {
			return unicode.ReplacementChar
		}
		if unicode.IsPrint(r) || r == '\n' || r == '\r' || r == '\t' {
			return r
		}
		return unicode.ReplacementChar
	}, s)
}

// journalValueToString converts a single journalctl JSON value to a string.
// systemd's journalctl --output=json encodes non-UTF8 / binary fields as
// JSON arrays of byte values (numbers 0-255) instead of strings.
func journalValueToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		buf := make([]byte, 0, len(val))
		for _, elem := range val {
			f, ok := elem.(float64)
			if !ok || f < 0 || f > math.MaxUint8 || f != math.Trunc(f) {
				// Not a valid byte array (mixed types, out-of-range, or
				// fractional values). Fall back to a textual representation
				// so the proto string field is always populated and the
				// server never crashes on unexpected input.
				return fmt.Sprintf("%v", v)
			}
			buf = append(buf, byte(f))
		}
		return sanitizeString(string(buf))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// journalEntryToStringMap converts a map[string]any (from JSON unmarshal) to
// map[string]string suitable for the JournalRecordRaw proto entry field.
func journalEntryToStringMap(raw map[string]any) map[string]string {
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = journalValueToString(v)
	}
	return out
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

	// Parse the output. journalctl --output=json emits one JSON object per line.
	// We deliberately avoid bufio.Scanner here: its default 64 KiB token cap makes
	// Scan stop on an oversized entry with bufio.ErrTooLong, and because that error
	// is easy to overlook the remainder of the stream is silently dropped. Reading
	// whole lines with no fixed cap (and explicitly flagging any entry we have to
	// truncate) keeps the output complete and makes truncation visible rather than
	// silent.
	reader := bufio.NewReader(run.Stdout)
	for {
		line, truncated, readErr := readJournalLine(reader, maxJournalEntryBytes)

		if len(line) > 0 || truncated {
			if truncated {
				// Fail loud but keep streaming: emit an explicit marker so a
				// caller can never mistake a dropped oversized entry for the
				// natural end of the logs.
				marker := fmt.Sprintf("[sansshell: journal entry truncated: exceeded %d bytes]", maxJournalEntryBytes)
				if err := sendJournalMarker(ctx, req, stream, marker); err != nil {
					return err
				}
			} else {
				var journalRaw map[string]any
				if err := json.Unmarshal(line, &journalRaw); err != nil {
					recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "parse_err"))
					return status.Errorf(codes.Internal, "parse the journal entry from json string to map err: %v", err)
				}
				if err := sendJournalEntry(ctx, req, stream, journalEntryToStringMap(journalRaw)); err != nil {
					return err
				}
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "read_err"))
			return status.Errorf(codes.Internal, "journal: read error: %v", readErr)
		}
	}
	return nil
}

// maxJournalEntryBytes bounds how many bytes of a single journal line we buffer.
// journalctl entries are normally small, but a hostile or pathological entry can
// be arbitrarily large; we cap per-entry memory and mark anything larger as
// truncated instead of failing or silently stopping.
const maxJournalEntryBytes = 8 << 20 // 8 MiB

// readJournalLine reads a single '\n'-terminated line from r with no fixed size
// cap (unlike bufio.Scanner, whose 64 KiB limit silently ends iteration). If the
// line exceeds max bytes it returns the first max bytes, reports truncated=true,
// and discards the remainder up to the next newline so streaming can continue
// with the following entry. Any trailing newline is stripped.
func readJournalLine(r *bufio.Reader, max int) (line []byte, truncated bool, err error) {
	var buf []byte
	for {
		chunk, e := r.ReadSlice('\n')
		if !truncated {
			if max > 0 && len(buf)+len(chunk) > max {
				if remaining := max - len(buf); remaining > 0 {
					buf = append(buf, chunk[:remaining]...)
				}
				truncated = true
			} else {
				buf = append(buf, chunk...)
			}
		}
		// ReadSlice returns ErrBufferFull when the delimiter was not found before
		// its internal buffer filled; keep reading the rest of the line.
		if e == bufio.ErrBufferFull {
			continue
		}
		for len(buf) > 0 && (buf[len(buf)-1] == '\n' || buf[len(buf)-1] == '\r') {
			buf = buf[:len(buf)-1]
		}
		return buf, truncated, e
	}
}

// sendJournalEntry streams a single parsed journal entry in either raw-JSON or
// default form, matching the request.
func sendJournalEntry(ctx context.Context, req *pb.JournalRequest, stream pb.SysInfo_JournalServer, journalMap map[string]string) error {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if req.EnableJson {
		journalRecordRaw := &pb.JournalRecordRaw{Entry: journalMap}
		if err := stream.Send(&pb.JournalReply{
			Response: &pb.JournalReply_JournalRaw{JournalRaw: journalRecordRaw},
		}); err != nil {
			recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "stream_send_err"))
			return status.Errorf(codes.Internal, "journal: send error %v", err)
		}
		return nil
	}

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
	if err := stream.Send(&pb.JournalReply{
		Response: &pb.JournalReply_Journal{Journal: journalRecord},
	}); err != nil {
		recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "stream_send_err"))
		return status.Errorf(codes.Internal, "journal: send error %v", err)
	}
	return nil
}

// sendJournalMarker streams a synthetic entry indicating that an oversized entry
// was truncated, so truncation is always visible to the caller.
func sendJournalMarker(ctx context.Context, req *pb.JournalRequest, stream pb.SysInfo_JournalServer, marker string) error {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if req.EnableJson {
		entry := map[string]string{"MESSAGE": marker, "__TRUNCATED": "true"}
		if err := stream.Send(&pb.JournalReply{
			Response: &pb.JournalReply_JournalRaw{JournalRaw: &pb.JournalRecordRaw{Entry: entry}},
		}); err != nil {
			recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "stream_send_err"))
			return status.Errorf(codes.Internal, "journal: send error %v", err)
		}
		return nil
	}
	record := &pb.JournalRecord{
		RealtimeTimestamp: timestamppb.Now(),
		Message:           marker,
	}
	if err := stream.Send(&pb.JournalReply{
		Response: &pb.JournalReply_Journal{Journal: record},
	}); err != nil {
		recorder.CounterOrLog(ctx, sysinfoJournalFailureCounter, 1, attribute.String("reason", "stream_send_err"))
		return status.Errorf(codes.Internal, "journal: send error %v", err)
	}
	return nil
}
