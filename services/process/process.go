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
	return entries, nil
}

func darwinPsParser(r io.Reader) (map[int64]*ProcessEntry, error) {
	entries := make(map[int64]*ProcessEntry)

	scanner := bufio.NewScanner(r)

	// Skip the first line of text since it's the header on OS/X
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
			Eip:             -1,
			Esp:             -1,
			CaughtSignals:   -1,
			IgnoredSignals:  -1,
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

		// Assume we increment but reset if we found a new start (i.e. not space in first position).
		// Lines look like
		//
		// user PID TT %CPU ...
		//      PID TT %CPU ...
		//
		// for something with 2 threads. Each blank leading line indicates another thread.

		// Only increment thread count if we've parsed the first one since otherwise it'll add one
		// spurious thread to the first entry.
		if numEntries > 0 {
			out.NumberOfThreads++
		}

		if len(fields) >= FIELD_COUNT {
			// The first line will match this and we haven't parsed yet so skip until it's done.
			// Also have to handle this below when we get done with the scanner.

			if numEntries > 0 { // Done counting for this entry so fill in number of threads and insert into the map.
				entries[out.Pid] = out
				out = blank()
			}
			numEntries++
		}

		// We only process the main line.
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
			default:
				out.State = ProcessState_PROCESS_STATE_UNKNOWN
			}

			// Everything left is the command so gather it up, rejoin with spaces
			// and we're done with this line.
			out.Command = strings.Join(fields[23:], " ")
		}

	}

	if err := scanner.Err(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("parsing error:\n%v", err))
	}

	// Final entry
	entries[out.Pid] = out

	return entries, nil
}

func (s *server) List(ctx context.Context, req *ListRequest) (*ListReply, error) {
	cmdName := *psBin
	options, ok := osSpecificPsFlags[runtime.GOOS]
	if !ok {
		return nil, status.Error(codes.Unimplemented, fmt.Sprintf("no support for OS %q", runtime.GOOS))
	}

	psOptions := options()

	log.Printf("Received request for List: %+v", req)
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
