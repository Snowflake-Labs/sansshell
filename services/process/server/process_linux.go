//go:build linux
// +build linux

package server

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative process.proto

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"strings"

	pb "github.com/Snowflake-Labs/sansshell/services/process"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	psBin     = flag.String("ps-bin", "/usr/bin/ps", "Path to the ps binary")
	pstackBin = flag.String("pstack-bin", "/usr/bin/pstack", "Path to the pstack binary")
	gcoreBin  = flag.String("gcore-bin", "/usr/bin/gcore", "Path to the gcore binary")

	// This is a var so we can replace for testing.
	psOptions = func() []string {
		options := []string{
			"pid",
			"ppid",
			"lwp",
			"wchan:32", // Make this wider than the default since kernel functions are often wordy.
			"pcpu",
			"pmem",
			"stime",
			"time",
			"rss",
			"vsz",
			"egid",
			"euid",
			"rgid",
			"ruid",
			"sgid",
			"suid",
			"nice",
			"priority",
			"class",
			"flag",
			"stat",
			"eip",
			"esp",
			"blocked",
			"caught",
			"ignored",
			"pending",
			"nlwp",
			"cmd", // Last so it's easy to parse.
		}

		return []string{
			"--noheader",
			"-e",
			"-o",
			strings.Join(options, ","),
		}
	}
)

func parser(r io.Reader) (map[int64]*pb.ProcessEntry, error) {
	entries := make(map[int64]*pb.ProcessEntry)

	scanner := bufio.NewScanner(r)
	out := &pb.ProcessEntry{}

	for scanner.Scan() {
		text := scanner.Text()
		fields := strings.Fields(text)

		// In case the last line is blank. In general a blank doesn't hurt.
		if len(text) == 0 {
			continue
		}

		// We know there are at least 28 fields but can be more
		// since command could have spaces.
		const FIELD_COUNT = 28

		if len(fields) < FIELD_COUNT {
			return nil, status.Errorf(codes.Internal, "invalid field count for line. Must be %d minimum. Input: %q", FIELD_COUNT, text)
		}

		for _, f := range []struct {
			name  string
			field string
			fmt   string
			out   interface{}
		}{
			{
				name:  "pid",
				field: fields[0],
				fmt:   "%d",
				out:   &out.Pid,
			},
			{
				name:  "ppid",
				field: fields[1],
				fmt:   "%d",
				out:   &out.Ppid,
			}, {
				name:  "lwp",
				field: fields[2],
				fmt:   "%d",
				out:   &out.ThreadId,
			},
			{
				name:  "wchan",
				field: fields[3],
				fmt:   "%s",
				out:   &out.Wchan,
			},
			{
				name:  "pcpu",
				field: fields[4],
				fmt:   "%f",
				out:   &out.CpuPercent,
			},
			{
				name:  "pmem",
				field: fields[5],
				fmt:   "%f",
				out:   &out.MemPercent,
			},
			{
				name:  "started time",
				field: fields[6],
				fmt:   "%s",
				out:   &out.StartedTime,
			},
			{
				name:  "elapsed time",
				field: fields[7],
				fmt:   "%s",
				out:   &out.ElapsedTime,
			},
			{
				name:  "rss",
				field: fields[8],
				fmt:   "%d",
				out:   &out.Rss,
			},
			{
				name:  "vsize",
				field: fields[9],
				fmt:   "%d",
				out:   &out.Vsize,
			},
			{
				name:  "egid",
				field: fields[10],
				fmt:   "%d",
				out:   &out.Egid,
			},
			{
				name:  "euid",
				field: fields[11],
				fmt:   "%d",
				out:   &out.Euid,
			},
			{
				name:  "rgid",
				field: fields[12],
				fmt:   "%d",
				out:   &out.Rgid,
			},
			{
				name:  "ruid",
				field: fields[13],
				fmt:   "%d",
				out:   &out.Ruid,
			},
			{
				name:  "sgid",
				field: fields[14],
				fmt:   "%d",
				out:   &out.Sgid,
			},
			{
				name:  "suid",
				field: fields[15],
				fmt:   "%d",
				out:   &out.Suid,
			},

			{
				name:  "priority",
				field: fields[17],
				fmt:   "%d",
				out:   &out.Priority,
			},
			{
				name:  "flags",
				field: fields[19],
				fmt:   "%x",
				out:   &out.Flags,
			},
			{
				name:  "eip",
				field: fields[21],
				fmt:   "%x",
				out:   &out.Eip,
			},
			{
				name:  "e2p",
				field: fields[22],
				fmt:   "%x",
				out:   &out.Esp,
			},
			{
				name:  "blocked",
				field: fields[23],
				fmt:   "%x",
				out:   &out.BlockedSignals,
			},
			{
				name:  "caught",
				field: fields[24],
				fmt:   "%x",
				out:   &out.CaughtSignals,
			},
			{
				name:  "ignored",
				field: fields[25],
				fmt:   "%x",
				out:   &out.IgnoredSignals,
			},
			{
				name:  "pending",
				field: fields[26],
				fmt:   "%x",
				out:   &out.PendingSignals,
			},
			{
				name:  "nlwp",
				field: fields[27],
				fmt:   "%d",
				out:   &out.NumberOfThreads,
			},
		} {
			if n, err := fmt.Sscanf(f.field, f.fmt, f.out); n != 1 || err != nil {
				return nil, status.Errorf(codes.Internal, "can't extract %s: %q in line %q: %v", f.name, f.field, text, err)
			}
		}

		// Need to parse nice directly since for some scheduling classes it can be a - and isn't a number.
		// In that case we leave it 0 and it's up to clients to display as ps would with a -.
		if fields[16] != "-" {
			if n, err := fmt.Sscanf(fields[16], "%d", &out.Nice); n != 1 || err != nil {
				return nil, status.Errorf(codes.Internal, "can't extract nice: %q in line %q: %v", fields[16], text, err)
			}
		}

		// Class and stat have to be direct parsed.
		const SCHEDULE_CLASS = 18
		const STATE = 20

		// Class
		switch fields[SCHEDULE_CLASS] {
		case "-":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_NOT_REPORTED
		case "TS":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_OTHER
		case "FF":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_FIFO
		case "RR":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_RR
		case "B":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_BATCH
		case "ISO":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_ISO
		case "IDL":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_ISO
		case "DLN":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_DEADLINE
		case "?":
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_UNKNOWN
		default:
			out.SchedulingClass = pb.SchedulingClass_SCHEDULING_CLASS_UNKNOWN
		}

		// State
		switch fields[STATE][0] {
		case 'D':
			out.State = pb.ProcessState_PROCESS_STATE_UNINTERRUPTIBLE_SLEEP
		case 'R':
			out.State = pb.ProcessState_PROCESS_STATE_RUNNING
		case 'S':
			out.State = pb.ProcessState_PROCESS_STATE_INTERRUPTIBLE_SLEEP
		case 'T':
			out.State = pb.ProcessState_PROCESS_STATE_STOPPED_JOB_CONTROL
		case 't':
			out.State = pb.ProcessState_PROCESS_STATE_STOPPED_DEBUGGER
		case 'Z':
			out.State = pb.ProcessState_PROCESS_STATE_ZOMBIE
		}

		// Now process any trailing symbols on State
		for i := 1; i < len(fields[STATE]); i++ {
			switch fields[STATE][i] {
			case '<':
				out.StateCode = append(out.StateCode, pb.ProcessStateCode_PROCESS_STATE_CODE_HIGH_PRIORITY)
			case 'N':
				out.StateCode = append(out.StateCode, pb.ProcessStateCode_PROCESS_STATE_CODE_LOW_PRIORITY)
			case 'L':
				out.StateCode = append(out.StateCode, pb.ProcessStateCode_PROCESS_STATE_CODE_LOCKED_PAGES)
			case 's':
				out.StateCode = append(out.StateCode, pb.ProcessStateCode_PROCESS_STATE_CODE_SESSION_LEADER)
			case 'l':
				out.StateCode = append(out.StateCode, pb.ProcessStateCode_PROCESS_STATE_CODE_MULTI_THREADED)
			case '+':
				out.StateCode = append(out.StateCode, pb.ProcessStateCode_PROCESS_STATE_CODE_FOREGROUND_PGRP)
			default:
				out.StateCode = append(out.StateCode, pb.ProcessStateCode_PROCESS_STATE_CODE_UNKNOWN)
			}
		}

		// Everything left is the command so gather it up, rejoin with spaces
		// and we're done with this line.
		out.Command = strings.Join(fields[FIELD_COUNT:], " ")
		entries[out.Pid] = out
		out = &pb.ProcessEntry{}
	}

	if err := scanner.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "parsing error:\n%v", err)
	}

	if len(entries) == 0 {
		return nil, status.Error(codes.Internal, "no output from ps?")
	}

	return entries, nil
}
