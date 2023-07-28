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
	"strconv"
	"time"

	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/euank/go-kmsg-parser/v2/kmsgparser"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// for testing
var getKmsgParser = func() (kmsgparser.Parser, error) {
	return kmsgparser.NewParser()
}

// provide journal interface for testing purpose
// in testing file, we can mock these functions
type journal interface {
	SeekTail() error
	Close() error
	Previous() (uint64, error)
	GetEntry() (*sdjournal.JournalEntry, error)
	GetCatalog() (string, error)
}

var getJournal = func() (journal, error) {
	return sdjournal.NewJournal()
}

var getUptime = func() (time.Duration, error) {
	sysinfo := &unix.Sysinfo_t{}
	if err := unix.Sysinfo(sysinfo); err != nil {
		return 0, status.Errorf(codes.Internal, "err in get the system info from unix")
	}
	uptime := time.Duration(sysinfo.Uptime) * time.Second
	return uptime, nil
}

// Based on: https://pkg.go.dev/github.com/euank/go-kmsg-parser
// kmsg-parser only allows us to read message from /dev/kmsg in blocking way
// we set 2 seconds timeout to explicitly close the channel
// If the package release new feature to support non-blocking read, we can
// make corresding changes below to get rid of hard code timeout setting
var getKernelMessages = func() ([]*pb.DmsgRecord, error) {
	parser, err := getKmsgParser()
	if err != nil {
		return nil, err
	}

	var records []*pb.DmsgRecord
	messages := parser.Parse()
	done := false
	for !done {
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
		case <-time.After(2 * time.Second):
			parser.Close()
			done = true
		}
	}
	return records, nil
}

var getJournalRecords = func(req *pb.JournalRequest) ([]*pb.JournalReply, error) {
	journal, err := getJournal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sdjournal initializes Journal instance error: %v", err)
	}
	defer journal.Close()

	// Seek to the end of the journal entries
	journal.SeekTail()

	var records []*pb.JournalReply
	for {
		// if collect expected log entries, break the loop
		// if req.TailLine is a negative number or larger than number of entries in journal
		// just fetch all journal entries
		if len(records) == int(req.TailLine) {
			break
		}
		// move read pointer back by on entry
		n, err := journal.Previous()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "sdjournal advances read pointer error: %v", err)
		}
		// break if there are no more entries left
		if n == 0 {
			break
		}

		entry, err := journal.GetEntry()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "sdjournal gets journal entry error: %v", err)
		}

		// filter based on input time
		realtime := timestamppb.New(time.Unix(0, int64(entry.RealtimeTimestamp)*int64(time.Microsecond)))
		// req.TimeSince <= realtime <= req.TimeUntil
		if req.TimeSince != nil && req.TimeSince.AsTime().After(realtime.AsTime()) {
			continue
		}
		if req.TimeUntil != nil && req.TimeUntil.AsTime().Before(realtime.AsTime()) {
			continue
		}

		// filter based on systemd unit
		// why `UNIT`, refer to doc: https://man.archlinux.org/man/systemd.journal-fields.7.en
		if unit, systemdUnit := entry.Fields["UNIT"], entry.Fields[sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT]; req.Unit != "" &&
			unit != req.Unit && systemdUnit != req.Unit {
			continue
		}

		switch req.Output {
		case "json", "json-pretty":
			journalRecordRaw := &pb.JournalRecordRaw{}
			journalRecordRaw.Entry = entry.Fields
			entryMap := journalRecordRaw.Entry
			// add cursor, realtime_timestamp and monotonic_timestamp to map
			entryMap[sdjournal.SD_JOURNAL_FIELD_CURSOR] = entry.Cursor
			entryMap[sdjournal.SD_JOURNAL_FIELD_REALTIME_TIMESTAMP] = strconv.FormatUint(entry.RealtimeTimestamp, 10)
			entryMap[sdjournal.SD_JOURNAL_FIELD_MONOTONIC_TIMESTAMP] = strconv.FormatUint(entry.MonotonicTimestamp, 10)
			records = append(records, &pb.JournalReply{
				Response: &pb.JournalReply_JournalRaw{
					JournalRaw: journalRecordRaw,
				},
			})
		case "":
			// default format
			journalRecord := &pb.JournalRecord{}
			journalRecord.RealtimeTimestamp = realtime
			journalRecord.Hostname = entry.Fields[sdjournal.SD_JOURNAL_FIELD_HOSTNAME]
			journalRecord.SyslogIdentifier = entry.Fields[sdjournal.SD_JOURNAL_FIELD_SYSLOG_IDENTIFIER]
			journalRecord.Message = entry.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE]

			// some log entries may not have pid, since they are not generated by a process
			if pidStr, ok := entry.Fields[sdjournal.SD_JOURNAL_FIELD_PID]; ok {
				pid, err := strconv.Atoi(pidStr)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "sdjournal pid converts error: %v from string to int32", err)
				}
				journalRecord.Pid = int32(pid)
			}

			// augument the records
			// can only add explanatory text for those log entries that have `MESSAGE_ID` field
			if _, ok := entry.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE_ID]; ok && req.Explain {
				catalog, err := journal.GetCatalog()
				if err != nil {
					return nil, status.Errorf(codes.Internal, "sdjournal getCatalog error: %v", err)
				}
				journalRecord.Catalog = catalog
			}
			records = append(records, &pb.JournalReply{
				Response: &pb.JournalReply_Journal{
					Journal: journalRecord,
				},
			})
		}
	}
	return records, nil
}
