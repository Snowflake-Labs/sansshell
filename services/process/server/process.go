package server

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server is used to implement the gRPC server
type server struct {
}

var (
	// These are effectively platform agnostic so they can here vs the architecture specific files.
	jstackBin = flag.String("jstack-bin", "/usr/lib/jvm/adoptopenjdk-11-hotspot/bin/jstack", "Path to the jstack binary")
	jmapBin   = flag.String("jmap-bin", "/usr/lib/jvm/adoptopenjdk-11-hotspot/bin/jmap", "Path to the jmap binary")
)

// The maximum we should allow stdout or stderr to be when sending back in an error string.
// grpc has limits on how large a returned error can be (generally 4-8k depending on language).
const MAX_BUF = 1024

func trimString(s string) string {
	if len(s) > MAX_BUF {
		s = s[:MAX_BUF]
	}
	return s
}

// Vars so we can replace for testing.
var (
	pstackOptions = func(req *pb.GetStacksRequest) []string {
		return []string{
			fmt.Sprintf("%d", req.Pid),
		}
	}

	jstackOptions = func(req *pb.GetJavaStacksRequest) []string {
		return []string{
			fmt.Sprintf("%d", req.Pid),
		}
	}
)

func (s *server) List(ctx context.Context, req *pb.ListRequest) (*pb.ListReply, error) {
	log.Printf("Received request for List: %+v", req)

	cmdName := *psBin
	options := psOptions()

	// We gather all the processes up and then filter by pid if needed at the end.
	cmd := exec.CommandContext(ctx, cmdName, options...)
	var stderrBuf, stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := cmd.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "command exited with error: %v\n%s", err, trimString(stderrBuf.String()))
	}

	if len(stderrBuf.String()) != 0 {
		return nil, status.Errorf(codes.Internal, "unexpected error output:\n%s", trimString(stderrBuf.String()))
	}

	entries, err := parser(&stdoutBuf)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "unexpected parsing error: %v", err)
	}

	reply := &pb.ListReply{}
	if len(req.Pids) != 0 {
		for _, pid := range req.Pids {
			if _, ok := entries[pid]; !ok {
				return nil, status.Errorf(codes.InvalidArgument, "pid %d does not exist", pid)
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

func (s *server) GetStacks(ctx context.Context, req *pb.GetStacksRequest) (*pb.GetStacksReply, error) {
	log.Printf("Received request for GetStacks: %+v", req)

	// This is tied to pstack so either an OS provides it or it doesn't.
	if *pstackBin == "" {
		return nil, status.Error(codes.Unimplemented, "not implemented")
	}

	if req.Pid <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pid must be non-zero and positive")
	}

	// TODO(jchacon): Push all of this into a util library which validates things and forces
	//                commands to have an abs path (testing will have to resolve path first).
	cmdName := *pstackBin
	options := pstackOptions(req)

	cmd := exec.CommandContext(ctx, cmdName, options...)
	var stderrBuf, stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := cmd.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "command exited with error: %v\n%s", err, trimString(stderrBuf.String()))
	}

	if len(stderrBuf.String()) != 0 {
		return nil, status.Errorf(codes.Internal, "unexpected error output:\n%s", trimString(stderrBuf.String()))
	}

	scanner := bufio.NewScanner(&stdoutBuf)
	out := &pb.GetStacksReply{}

	numEntries := 0
	stack := &pb.ThreadStack{}

	for scanner.Scan() {
		text := scanner.Text()
		fields := strings.Fields(text)

		// Blank lines don't hurt, just skip.
		if len(fields) == 0 {
			continue
		}

		// New thread so append last entry and start over.
		if fields[0] == "Thread" {
			if len(fields) != 6 {
				return nil, status.Errorf(codes.Internal, "unparsable pstack output for new thread: %s", text)
			}

			if numEntries > 0 {
				out.Stacks = append(out.Stacks, stack)
				stack = &pb.ThreadStack{}
			}

			if n, err := fmt.Sscanf(fields[1], "%d", &stack.ThreadNumber); n != 1 || err != nil {
				return nil, status.Errorf(codes.Internal, "can't parse thread number: %s : %v", text, err)
			}
			if n, err := fmt.Sscanf(fields[3], "0x%x", &stack.ThreadId); n != 1 || err != nil {
				return nil, status.Errorf(codes.Internal, "can't parse thread id: %s : %v", text, err)
			}
			if n, err := fmt.Sscanf(fields[5], "%d", &stack.Lwp); n != 1 || err != nil {
				return nil, status.Errorf(codes.Internal, "can't parse lwp: %s : %v", text, err)
			}
			numEntries++
			continue
		}

		// Anything else is a stack entry so just append it.
		stack.Stacks = append(stack.Stacks, text)
	}

	// Append last entry
	out.Stacks = append(out.Stacks, stack)
	return out, nil
}

func (s *server) GetJavaStacks(ctx context.Context, req *pb.GetJavaStacksRequest) (*pb.GetJavaStacksReply, error) {
	log.Printf("Received request for GetJavaStacks: %+v", req)

	// This is tied to pstack so either an OS provides it or it doesn't.
	if *jstackBin == "" {
		return nil, status.Error(codes.Unimplemented, "not implemented")
	}

	if req.Pid <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pid must be non-zero and positive")
	}

	cmdName := *jstackBin
	options := jstackOptions(req)

	cmd := exec.CommandContext(ctx, cmdName, options...)
	var stderrBuf, stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// jstack emits stderr output related to environment vars. So only complain on a non-zero exit.
	if err := cmd.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "command exited with error: %v\n%s", err, trimString(stderrBuf.String()))
	}

	scanner := bufio.NewScanner(&stdoutBuf)
	out := &pb.GetJavaStacksReply{}

	numEntries := 0
	stack := &pb.JavaThreadStack{}

	for scanner.Scan() {
		text := scanner.Text()

		// Just skip blank lines
		if len(text) == 0 {
			continue
		}

		if text[0] != '"' { // Anything else is a stack entry so just append it.
			stack.Stacks = append(stack.Stacks, text)
			continue
		}
		// Start of a new entry. Push the old one into the reply.
		if numEntries > 0 {
			out.Stacks = append(out.Stacks, stack)
			stack = &pb.JavaThreadStack{}
		}
		numEntries++

		// Find the trailing " character to extact the name.
		end := strings.Index(text[1:], `"`)
		if end == -1 {
			return nil, status.Errorf(codes.Internal, "can't find thread name in line %q", text)
		}

		// Remember end is offset by one due to skipping the first " char above.
		end++
		stack.Name = text[1:end]

		// Split the remaining fields up
		end++
		fields := strings.Fields(text[end:])

		// If it's a daemon that's in the 2nd field (or not)
		if fields[1] == "daemon" {
			stack.Daemon = true
			// Remove that field and shift over so parsing below is simpler.
			copy(fields[1:], fields[2:])
			fields = fields[0 : len(fields)-1]
		}

		// A Java thread has a thread number and more details. Other ones represent native/C++ threads
		// which expose no stacks and have less fields.
		//
		// Java thread:
		//
		// "Attach Listener" #19 daemon prio=9 os_prio=0 cpu=1.19ms elapsed=4723.25s tid=0x00007f7818001000 nid=0x5606 waiting on condition  [0x0000000000000000]
		//
		// Native/C++ thread:
		//
		// "G1 Refine#0" os_prio=0 cpu=1.63ms elapsed=3042612.92s tid=0x00007f787826b000 nid=0x7eed runnable
		var state []string
		for _, f := range fields {
			var format string
			var out interface{}
			switch {
			case strings.HasPrefix(f, "#"):
				format = "#%d"
				out = &stack.ThreadNumber
			case strings.HasPrefix(f, "prio="):
				format = "prio=%d"
				out = &stack.Priority
			case strings.HasPrefix(f, "os_prio="):
				format = "os_prio=%d"
				out = &stack.OsPriority
			case strings.HasPrefix(f, "cpu="):
				format = "cpu=%fms"
				out = &stack.CpuMs
			case strings.HasPrefix(f, "elapsed="):
				format = "elapsed=%fs"
				out = &stack.ElapsedSec
			case strings.HasPrefix(f, "tid="):
				format = "tid=0x%x"
				out = &stack.ThreadId
			case strings.HasPrefix(f, "nid="):
				format = "nid=0x%x"
				out = &stack.NativeThreadId
			case strings.HasPrefix(f, "[0x"):
				format = "[0x%x]"
				out = &stack.Pc
			default:
				state = append(state, f)
				continue
			}
			if n, err := fmt.Sscanf(f, format, out); n != 1 || err != nil {
				return nil, status.Errorf(codes.Internal, "can't parse %q out of text: %q - %v", format, text, err)
			}
		}
		stack.State = strings.Join(state, " ")
	}

	// Append last entry
	out.Stacks = append(out.Stacks, stack)
	return out, nil
}

func (s *server) GetCore(req *pb.GetCoreRequest, stream pb.Process_GetCoreServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

func (s *server) GetJavaHeapDump(req *pb.GetJavaHeapDumpRequest, stream pb.Process_GetJavaHeapDumpServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterProcessServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
