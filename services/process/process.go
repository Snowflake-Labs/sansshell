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
		"command",
	}

	return []string{
		"-M",
		"-e",
		"-o",
		strings.Join(psOptions, ","),
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

	// Always start with one thread as the default.
	numThreads := int64(0)
	numEntries := 0

	out := &ProcessEntry{}

	for scanner.Scan() {
		// We should get back exactly the same amount of fields as we asked but
		// cmd can have spaces so stop before it.
		// Also on darwin the only way to get a thread count if with -M which adds
		// 9 columns regardless of -o (they get added after it). One of those 9 is
		// command which means it can have spaces...So parsing is painful.
		text := scanner.Text()

		// Assume we increment but reset if we found a new start (i.e. not space in first position).
		// Lines look like
		//
		// user PID TT %CPU ...
		//      PID TT %CPU ...
		//
		// for something with 2 threads. Each blank leading line indicates another thread.
		numThreads++
		if text[0] != ' ' {
			// The first line will match this and we haven't parsed yet so skip until it's done.
			// Also have to handle this below when we get done with the scanner.

			if numEntries > 0 { // Done counting for this entry so fill in number of threads and insert into the map.
				out.NumberOfThreads = numThreads
				entries[out.Pid] = out
				log.Printf("Got entry: %+v", out)
				out = &ProcessEntry{}
				numThreads = 1
			}
			numEntries++
		}

		// We only process the main line.
		if numThreads == 1 {
			// Take the line and split by spaces
			lineScanner := bufio.NewScanner(strings.NewReader(text))
			lineScanner.Split(bufio.ScanWords)

			// Helper for moving the scanner forward and checking errors.
			sc := func(reason string) error {
				if !lineScanner.Scan() {
					err := lineScanner.Err()
					return status.Error(codes.Internal, fmt.Sprintf("parse error against %q: reason %s - %v", text, reason, err))
				}
				return nil
			}

			// First field is username, skip it.
			if err := sc("username"); err != nil {
				return nil, err
			}

			// 2nd is pid, remember it.
			if err := sc("pid"); err != nil {
				return nil, err
			}
			pid := lineScanner.Text()

			// Now scan forward until we find pid again. Now we're in the fields we specified.
			for {
				// We either find the pid or run out of input.
				if err := sc("find pid again"); err != nil {
					return nil, err
				}
				if pid == lineScanner.Text() {
					break
				}
			}

			// Parse pid
			if n, err := fmt.Sscanf(pid, "%d", &out.Pid); n != 1 || err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("can't extract pid: %q in line %q: %v", pid, text, err))
			}

			// Helper func to extract a token and convert into the right field.
			parse := func(debug string, format string, dest interface{}) error {
				if err := sc("find " + debug); err != nil {
					return err
				}
				t := lineScanner.Text()
				if n, err := fmt.Sscanf(t, format, dest); n != 1 || err != nil {
					return status.Error(codes.Internal, fmt.Sprintf("can't extract %s: %q in line %q: %v", debug, t, text, err))
				}
				return nil
			}

			// Parse ppid
			if err := parse("ppid", "%d", &out.Ppid); err != nil {
				return nil, err
			}

			// Parse wchan
			if err := parse("wchan", "%s", &out.Wchan); err != nil {
				return nil, err
			}

			// Parse pcpu
			if err := parse("pcpu", "%f", &out.CpuPercent); err != nil {
				return nil, err
			}

			// Parse pmem
			if err := parse("pmem", "%f", &out.MemPercent); err != nil {
				return nil, err
			}

			// Parse started time
			if err := parse("started time", "%s", &out.StartedTime); err != nil {
				return nil, err
			}

			// Parse elapsed time
			if err := parse("elapsed time", "%s", &out.ElapsedTime); err != nil {
				return nil, err
			}

			// Parse rss
			if err := parse("rss", "%d", &out.Rss); err != nil {
				return nil, err
			}

			// Parse vsize
			if err := parse("vsize", "%d", &out.Vsize); err != nil {
				return nil, err
			}

			// Parse egid
			if err := parse("egid", "%d", &out.Egid); err != nil {
				return nil, err
			}

			// Parse euid
			if err := parse("euid", "%d", &out.Euid); err != nil {
				return nil, err
			}

			// Parse rgid
			if err := parse("rgid", "%d", &out.Rgid); err != nil {
				return nil, err
			}

			// Parse ruid
			if err := parse("ruid", "%d", &out.Ruid); err != nil {
				return nil, err
			}

			// Parse nice
			if err := parse("nice", "%d", &out.Nice); err != nil {
				return nil, err
			}

			// Parse priority. On darwin this has a trailing letter which isn't documented
			// so it's going to get ignored on the parse.
			if err := parse("priority", "%d", &out.Priority); err != nil {
				return nil, err
			}

			// Parse flags
			if err := parse("flags", "%x", &out.Flags); err != nil {
				return nil, err
			}

			// State has to be computed by hand.
			if err := sc("find state"); err != nil {
				return nil, err
			}
			t := lineScanner.Text()

			switch t[0] {
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

			// Parse blocked
			if err := parse("blocked", "%x", &out.BlockedSignals); err != nil {
				return nil, err
			}

			// Parse pending
			if err := parse("pending", "%x", &out.PendingSignals); err != nil {
				return nil, err
			}

			// Everything left is the command so gather it up, rejoin with spaces
			// and we're done with this line.
			var command []string
			for lineScanner.Scan() {
				command = append(command, lineScanner.Text())
			}
			out.Command = strings.Join(command, " ")
			if err := lineScanner.Err(); err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("parse error filling command %q: %v", text, err))
			}
		}

	}

	if err := scanner.Err(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("parsing error:\n%v", err))
	}

	// Final entry
	out.NumberOfThreads = numThreads
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
