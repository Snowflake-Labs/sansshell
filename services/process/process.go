package process

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative process.proto

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Snowflake-Labs/sansshell/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var psBin = flag.String("ps_bin", "/usr/bin/ps", "Location of the ps command")

// server is used to implement the gRPC server
type server struct {
}

var (
	// Keyyed by $GOOS an arch specific function for which flags to send ps.
	osSpecificPsFlags = map[string]func() []string{
		"linux":  linuxPsFlags,
		"darwin": darwinPsFlags,
	}

	// Keyyed by $GOOS an arch specific function to parse ps output and return
	// a map of ProcessEntry.
	osPsParser = map[string]func(io.Reader) (map[int64]*ProcessEntry, error){
		"linux":  linuxPsParser,
		"darwin": darwinPsParser,
	}
)

func linuxPsFlags() []string {
	psOptions := []string{
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
		strings.Join(psOptions, ","),
	}
}

func darwinPsFlags() []string {
	psOptions := []string{
		"pid",
		"ppid",
		"wchan",
		"pcpu",
		"pmem",
		"start",
		"time",
		"rss",
		"vsz",
		"gid",
		"uid",
		"rgid",
		"ruid",
		"nice",
		"pri",
		"flags",
		"stat",
		"blocked",
		"pending",
		"tt",
		"stime",
		"utime",
		"user",
		"command",
	}

	return []string{
		// Order here is important. -M must come after -o or it'll clip command and add fields it insists
		// must be there. It's why we include tt, stime, utime and user above even though we don't return
		// them in a ProcessEntry.
		"-e",
		"-o",
		strings.Join(psOptions, ","),
		"-M",
	}
}

func linuxPsParser(r io.Reader) (map[int64]*ProcessEntry, error) {
	entries := make(map[int64]*ProcessEntry)

	scanner := bufio.NewScanner(r)
	out := &ProcessEntry{}

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
			return nil, status.Error(codes.Internal, fmt.Sprintf("invalid field count for line. Must be %d minimum. Input: %q", FIELD_COUNT, text))
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
				return nil, status.Error(codes.Internal, fmt.Sprintf("can't extract %s: %q in line %q: %v", f.name, f.field, text, err))
			}
		}

		// Need to parse nice directly since for some scheduling classes it can be a - and isn't a number.
		// In that case we leave it 0 and it's up to clients to display as ps would with a -.
		if fields[16] != "-" {
			if n, err := fmt.Sscanf(fields[16], "%d", &out.Nice); n != 1 || err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("can't extract nice: %q in line %q: %v", fields[16], text, err))
			}
		}

		// Class and stat have to be direct parsed.
		const SCHEDULE_CLASS = 18
		const STATE = 20

		// Class
		switch fields[SCHEDULE_CLASS] {
		case "-":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_NOT_REPORTED
		case "TS":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_OTHER
		case "FF":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_FIFO
		case "RR":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_RR
		case "B":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_BATCH
		case "ISO":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_ISO
		case "IDL":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_ISO
		case "DLN":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_DEADLINE
		case "?":
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_UNKNOWN
		default:
			out.SchedulingClass = SchedulingClass_SCHEDULING_CLASS_UNKNOWN
		}

		// State
		switch fields[STATE][0] {
		case 'D':
			out.State = ProcessState_PROCESS_STATE_UNINTERRUPTIBLE_SLEEP
		case 'R':
			out.State = ProcessState_PROCESS_STATE_RUNNING
		case 'S':
			out.State = ProcessState_PROCESS_STATE_INTERRUPTIBLE_SLEEP
		case 'T':
			out.State = ProcessState_PROCESS_STATE_STOPPED_JOB_CONTROL
		case 't':
			out.State = ProcessState_PROCESS_STATE_STOPPED_DEBUGGER
		case 'Z':
			out.State = ProcessState_PROCESS_STATE_ZOMBIE
		default:
			out.State = ProcessState_PROCESS_STATE_UNKNOWN
		}

		// Now process any trailing symbols on State
		for i := 1; i < len(fields[STATE]); i++ {
			switch fields[STATE][i] {
			case '<':
				out.StateCode = append(out.StateCode, ProcessStateCode_PROCESS_STATE_CODE_HIGH_PRIORITY)
			case 'N':
				out.StateCode = append(out.StateCode, ProcessStateCode_PROCESS_STATE_CODE_LOW_PRIORITY)
			case 'L':
				out.StateCode = append(out.StateCode, ProcessStateCode_PROCESS_STATE_CODE_LOCKED_PAGES)
			case 's':
				out.StateCode = append(out.StateCode, ProcessStateCode_PROCESS_STATE_CODE_SESSION_LEADER)
			case 'l':
				out.StateCode = append(out.StateCode, ProcessStateCode_PROCESS_STATE_CODE_MULTI_THREADED)
			case '+':
				out.StateCode = append(out.StateCode, ProcessStateCode_PROCESS_STATE_CODE_FOREGROUND_PGRP)
			default:
				out.StateCode = append(out.StateCode, ProcessStateCode_PROCESS_STATE_CODE_UNKNOWN)
			}
		}

		// Everything left is the command so gather it up, rejoin with spaces
		// and we're done with this line.
		out.Command = strings.Join(fields[FIELD_COUNT:], " ")
		entries[out.Pid] = out
		out = &ProcessEntry{}
	}

	if err := scanner.Err(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("parsing error:\n%v", err))
	}

	if len(entries) == 0 {
		return nil, status.Error(codes.Internal, "no output from ps?")
	}

	return entries, nil
}

func darwinPsParser(r io.Reader) (map[int64]*ProcessEntry, error) {
	entries := make(map[int64]*ProcessEntry)

	scanner := bufio.NewScanner(r)

	// Skip the first line of text since it's the header on OS/X which
	// can't be eliminated via options.
	if !scanner.Scan() {
		err := scanner.Err()
		return nil, status.Error(codes.Internal, fmt.Sprintf("missing first line? %v", err))
	}

	numEntries := 0

	blank := func() *ProcessEntry {
		// Set the entries we know darwin doesn't fill in.
		return &ProcessEntry{
			Sgid:            -1,
			Suid:            -1,
			SchedulingClass: SchedulingClass_SCHEDULING_CLASS_UNKNOWN,
			NumberOfThreads: 1,
		}
	}

	out := blank()

	for scanner.Scan() {
		// We should get back exactly the same amount of fields as we asked but
		// cmd can have spaces so stop before it.
		// In this case we know it's >= 23 fields.
		const FIELD_COUNT = 23

		text := scanner.Text()
		fields := strings.Fields(text)

		// In case the last line is blank. In general a blank doesn't hurt.
		if len(text) == 0 {
			continue
		}

		// Assume we increment but reset if we found a new start (i.e. not space in first position).
		// Lines look like
		//
		// user PID TT %CPU ...
		//      PID TT %CPU ...
		//
		// for something with 2 threads. Each blank leading line indicates another thread.

		if len(fields) >= FIELD_COUNT {
			// The first line will match this and we haven't parsed yet so skip until it's done.
			// Also have to handle this below when we get done with the scanner.

			if numEntries > 0 { // Done counting for this entry so fill in number of threads and insert into the map.
				entries[out.Pid] = out
				out = blank()
			}
			numEntries++
		}

		// We only fully process the main line.
		if len(fields) >= FIELD_COUNT {
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
				},
				{
					name:  "wchan",
					field: fields[2],
					fmt:   "%s",
					out:   &out.Wchan,
				},
				{
					name:  "pcpu",
					field: fields[3],
					fmt:   "%f",
					out:   &out.CpuPercent,
				},
				{
					name:  "pmem",
					field: fields[4],
					fmt:   "%f",
					out:   &out.MemPercent,
				},
				{
					name:  "started time",
					field: fields[5],
					fmt:   "%s",
					out:   &out.StartedTime,
				},
				{
					name:  "elapsed time",
					field: fields[6],
					fmt:   "%s",
					out:   &out.ElapsedTime,
				},
				{
					name:  "rss",
					field: fields[7],
					fmt:   "%d",
					out:   &out.Rss,
				},
				{
					name:  "vsize",
					field: fields[8],
					fmt:   "%d",
					out:   &out.Vsize,
				},
				{
					name:  "egid",
					field: fields[9],
					fmt:   "%d",
					out:   &out.Egid,
				},
				{
					name:  "euid",
					field: fields[10],
					fmt:   "%d",
					out:   &out.Euid,
				},
				{
					name:  "rgid",
					field: fields[11],
					fmt:   "%d",
					out:   &out.Rgid,
				},
				{
					name:  "ruid",
					field: fields[12],
					fmt:   "%d",
					out:   &out.Ruid,
				},
				{
					name:  "nice",
					field: fields[13],
					fmt:   "%d",
					out:   &out.Nice,
				},
				{
					// On darwin this has a trailing letter which isn't documented
					// so it's going to get ignored on the parse.
					name:  "priority",
					field: fields[14],
					fmt:   "%d",
					out:   &out.Priority,
				},
				{
					name:  "flags",
					field: fields[15],
					fmt:   "%x",
					out:   &out.Flags,
				},
				{
					name:  "blocked",
					field: fields[17],
					fmt:   "%x",
					out:   &out.BlockedSignals,
				},
				{
					name:  "pending",
					field: fields[18],
					fmt:   "%x",
					out:   &out.PendingSignals,
				},
			} {
				if n, err := fmt.Sscanf(f.field, f.fmt, f.out); n != 1 || err != nil {
					return nil, status.Error(codes.Internal, fmt.Sprintf("can't extract %s: %q in line %q: %v", f.name, f.field, text, err))
				}
			}

			// State has to be computed by hand.
			switch fields[16][0] {
			case 'I', 'S':
				out.State = ProcessState_PROCESS_STATE_INTERRUPTIBLE_SLEEP
			case 'R':
				out.State = ProcessState_PROCESS_STATE_RUNNING
			case 'T':
				out.State = ProcessState_PROCESS_STATE_STOPPED_JOB_CONTROL
			case 'U':
				out.State = ProcessState_PROCESS_STATE_UNINTERRUPTIBLE_SLEEP
			case 'Z':
				out.State = ProcessState_PROCESS_STATE_ZOMBIE
			}

			// Everything left is the command so gather it up, rejoin with spaces
			// and we're done with this line.
			out.Command = strings.Join(fields[FIELD_COUNT:], " ")
		} else {
			// Bump the thread count
			out.NumberOfThreads++

			// Other lines may contain additional cpu/mem percent values we need to add up.
			var cpu, mem float32
			if n, err := fmt.Sscanf(fields[3], "%f", &cpu); n != 1 || err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("can't extract cpu in line %q: %v", text, err))
			}
			if n, err := fmt.Sscanf(fields[4], "%f", &mem); n != 1 || err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("can't extract mem in line %q: %v", text, err))
			}
			out.CpuPercent += cpu
			out.MemPercent += mem
		}

	}

	if err := scanner.Err(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("parsing error:\n%v", err))
	}

	// Final entry gets recorded.
	entries[out.Pid] = out

	if numEntries == 0 {
		return nil, status.Error(codes.Internal, "no output from ps?")
	}

	return entries, nil
}

func (s *server) List(ctx context.Context, req *ListRequest) (*ListReply, error) {
	log.Printf("Received request for List: %+v", req)

	cmdName := *psBin
	options, ok := osSpecificPsFlags[runtime.GOOS]
	if !ok {
		return nil, status.Error(codes.Unimplemented, fmt.Sprintf("no support for OS %q", runtime.GOOS))
	}

	psOptions := options()

	// We gather all the processes up and then filter by pid if needed at the end.
	cmd := exec.CommandContext(ctx, cmdName, psOptions...)
	var stderrBuf, stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := cmd.Wait(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("command exited with error: %v", err))
	}

	errBuf := stderrBuf.Bytes()
	if len(errBuf) != 0 {
		return nil, status.Error(codes.Internal, fmt.Sprintf("unexpected error output:\n%s", string(errBuf)))
	}

	parser, ok := osPsParser[runtime.GOOS]
	if !ok {
		return nil, status.Error(codes.Unimplemented, fmt.Sprintf("no support for OS %q", runtime.GOOS))
	}

	entries, err := parser(&stdoutBuf)

	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("unexpected parsing error: %v", err))
	}

	reply := &ListReply{}
	if len(req.Pids) != 0 {
		for _, pid := range req.Pids {
			if _, ok := entries[pid]; !ok {
				return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("pid %d does not exist", pid))
			}

			reply.ProcessEntries = append(reply.ProcessEntries, entries[pid])
		}
		return reply, nil
	}

	// If not filtering fill everything in and return. We don't guarentee any ordering.
	for _, e := range entries {
		reply.ProcessEntries = append(reply.ProcessEntries, e)
	}
	return reply, nil
}

func (s *server) GetStacks(ctx context.Context, req *GetStacksRequest) (*GetStacksReply, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *server) GetJavaStacks(ctx context.Context, req *GetJavaStacksRequest) (*GetJavaStacksReply, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *server) GetCore(req *GetCoreRequest, stream Process_GetCoreServer) error {
	return status.Error(codes.Unimplemented, "")
}

func (s *server) GetJavaHeapDump(req *GetJavaHeapDumpRequest, stream Process_GetJavaHeapDumpServer) error {
	return status.Error(codes.Unimplemented, "")
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterProcessServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
