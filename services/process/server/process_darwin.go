//go:build darwin
// +build darwin

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
	psBin     = flag.String("ps-bin", "/bin/ps", "Path to the ps binary")
	pstackBin = flag.String("pstack-bin", "", "Path to the pstack binary")
	gcoreBin  = flag.String("gcore-bin", "", "Path to the gcore binary")

	// This is a var so we can replace for testing.
	psOptions = func() []string {
		options := []string{
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
			strings.Join(options, ","),
			"-M",
		}
	}
)

func parser(r io.Reader) (map[int64]*pb.ProcessEntry, error) {
	entries := make(map[int64]*pb.ProcessEntry)

	scanner := bufio.NewScanner(r)

	// Skip the first line of text since it's the header on OS/X which
	// can't be eliminated via options.
	if !scanner.Scan() {
		err := scanner.Err()
		return nil, status.Errorf(codes.Internal, "missing first line? %v", err)
	}

	numEntries := 0

	blank := func() *pb.ProcessEntry {
		// Set the entries we know darwin doesn't fill in.
		return &pb.ProcessEntry{
			Sgid:            -1,
			Suid:            -1,
			SchedulingClass: pb.SchedulingClass_SCHEDULING_CLASS_UNKNOWN,
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
					return nil, status.Errorf(codes.Internal, "can't extract %s: %q in line %q: %v", f.name, f.field, text, err)
				}
			}

			// State has to be computed by hand.
			switch fields[16][0] {
			case 'I', 'S':
				out.State = pb.ProcessState_PROCESS_STATE_INTERRUPTIBLE_SLEEP
			case 'R':
				out.State = pb.ProcessState_PROCESS_STATE_RUNNING
			case 'T':
				out.State = pb.ProcessState_PROCESS_STATE_STOPPED_JOB_CONTROL
			case 'U':
				out.State = pb.ProcessState_PROCESS_STATE_UNINTERRUPTIBLE_SLEEP
			case 'Z':
				out.State = pb.ProcessState_PROCESS_STATE_ZOMBIE
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
				return nil, status.Errorf(codes.Internal, "can't extract cpu in line %q: %v", text, err)
			}
			if n, err := fmt.Sscanf(fields[4], "%f", &mem); n != 1 || err != nil {
				return nil, status.Errorf(codes.Internal, "can't extract mem in line %q: %v", text, err)
			}
			out.CpuPercent += cpu
			out.MemPercent += mem
		}

	}

	if err := scanner.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "parsing error:\n%v", err)
	}

	// Final entry gets recorded.
	entries[out.Pid] = out

	if numEntries == 0 {
		return nil, status.Error(codes.Internal, "no output from ps?")
	}

	return entries, nil
}
