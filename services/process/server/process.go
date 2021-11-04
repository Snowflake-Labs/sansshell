package server

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/process"
	"github.com/Snowflake-Labs/sansshell/services/util"
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

	// This will return options passed to the gcore command and the path to the resulting core file.
	// The file will be placed in a temporary directory and that entire directory should be cleaned
	// by the caller.
	// TODO(jchacon): This is annoying as it requires a file in order to work. We should be able to
	//                stream the data using Googles opensource library: https://code.google.com/archive/p/google-coredumper/
	gcoreOptionsAndLocation = func(req *pb.GetCoreRequest) ([]string, string, error) {
		dir, err := os.MkdirTemp(os.TempDir(), "cores")
		if err != nil {
			return nil, "", err
		}
		file := filepath.Join(dir, "core")
		return []string{
			"-o",
			file,
			fmt.Sprintf("%d", req.Pid),
		}, fmt.Sprintf("%s.%d", file, req.Pid), nil
	}

	// This will return options passed to the gcore command and the path to the resulting core file.
	// The file will be placed in a temporary directory and that entire directory should be cleaned
	// by the caller.
	// TODO(jchacon): This is annoying as it requires a file in order to work. We should be able to
	//                stream the data somehow though that may require a private fork of jmap.
	jmapOptionsAndLocation = func(req *pb.GetJavaHeapDumpRequest) ([]string, string, error) {
		dir, err := os.MkdirTemp(os.TempDir(), "heaps")
		if err != nil {
			return nil, "", err
		}
		file := filepath.Join(dir, "heap.bin")
		return []string{
			fmt.Sprintf("-dump:format=b,file=%s", file),
			fmt.Sprintf("%d", req.Pid),
		}, file, nil
	}
)

func (s *server) List(ctx context.Context, req *pb.ListRequest) (*pb.ListReply, error) {
	log.Printf("Received request for List: %+v", req)

	cmdName := *psBin
	options := psOptions()

	// We gather all the processes up and then filter by pid if needed at the end.
	run, err := util.RunCommand(ctx, cmdName, options)
	if err != nil {
		return nil, err
	}

	if err := run.Error; err != nil {
		return nil, status.Errorf(codes.Internal, "command exited with error: %v\n%s", err, util.TrimString(run.Stderr.String()))
	}

	if len(run.Stderr.String()) != 0 {
		return nil, status.Errorf(codes.Internal, "unexpected error output:\n%s", util.TrimString(run.Stderr.String()))
	}

	entries, err := parser(run.Stdout)

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

	cmdName := *pstackBin
	options := pstackOptions(req)

	run, err := util.RunCommand(ctx, cmdName, options)
	if err != nil {
		return nil, err
	}

	if err := run.Error; err != nil {
		return nil, status.Errorf(codes.Internal, "command exited with error: %v\n%s", err, util.TrimString(run.Stderr.String()))
	}

	if len(run.Stderr.String()) != 0 {
		return nil, status.Errorf(codes.Internal, "unexpected error output:\n%s", util.TrimString(run.Stderr.String()))
	}

	scanner := bufio.NewScanner(run.Stdout)
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

	run, err := util.RunCommand(ctx, cmdName, options)
	if err != nil {
		return nil, err
	}

	// jstack emits stderr output related to environment vars. So only complain on a non-zero exit.
	if err := run.Error; err != nil {
		return nil, status.Errorf(codes.Internal, "command exited with error: %v\n%s", err, util.TrimString(run.Stderr.String()))
	}

	scanner := bufio.NewScanner(run.Stdout)
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
	log.Printf("Received request for GetCore: %+v", req)

	// This is tied to gcore so either an OS provides it or it doesn't.
	if *gcoreBin == "" {
		return status.Error(codes.Unimplemented, "not implemented")
	}

	if req.Pid <= 0 {
		return status.Error(codes.InvalidArgument, "pid must be non-zero and positive")
	}

	if req.Destination == pb.BlobDestination_BLOB_DESTINATION_UNKNOWN {
		req.Destination = pb.BlobDestination_BLOB_DESTINATION_STREAM
	}

	if req.Destination != pb.BlobDestination_BLOB_DESTINATION_STREAM {
		return status.Error(codes.Unimplemented, "only streaming destinations are implemented")
	}

	cmdName := *gcoreBin
	options, file, err := gcoreOptionsAndLocation(req)
	if err != nil {
		return status.Errorf(codes.Internal, "can't generate options/core file location: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(file)) // clean up

	run, err := util.RunCommand(stream.Context(), cmdName, options)
	if err != nil {
		return err
	}

	if err := run.Error; err != nil {
		return status.Errorf(codes.Internal, "command exited with error: %v\n%s", err, util.TrimString(run.Stderr.String()))
	}

	if len(run.Stderr.String()) != 0 {
		return status.Errorf(codes.Internal, "unexpected error output:\n%s", util.TrimString(run.Stderr.String()))
	}

	f, err := os.Open(file)
	if err != nil {
		return status.Errorf(codes.Internal, "can't open %s for processing: %v", file, err)
	}
	defer f.Close()

	b := make([]byte, util.StreamingChunkSize)
	for {
		n, err := f.Read(b)
		// We're done on EOF.
		if err == io.EOF {
			break
		}

		if err != nil {
			return status.Errorf(codes.Internal, "can't read file %s: %v", file, err)
		}

		// Only send over the number of bytes we actually read or
		// else we'll send over garbage in the last packet potentially.
		if err := stream.Send(&pb.GetCoreReply{Data: b[:n]}); err != nil {
			return status.Errorf(codes.Internal, "can't send on stream: %v", err)
		}
	}
	return nil
}

func (s *server) GetJavaHeapDump(req *pb.GetJavaHeapDumpRequest, stream pb.Process_GetJavaHeapDumpServer) error {
	log.Printf("Received request for GetJavaHeapDump: %+v", req)

	// This is tied to gcore so either an OS provides it or it doesn't.
	if *jmapBin == "" {
		return status.Error(codes.Unimplemented, "not implemented")
	}

	if req.Pid <= 0 {
		return status.Error(codes.InvalidArgument, "pid must be non-zero and positive")
	}

	if req.Destination == pb.BlobDestination_BLOB_DESTINATION_UNKNOWN {
		req.Destination = pb.BlobDestination_BLOB_DESTINATION_STREAM
	}

	if req.Destination != pb.BlobDestination_BLOB_DESTINATION_STREAM {
		return status.Error(codes.Unimplemented, "only streaming destinations are implemented")
	}

	cmdName := *jmapBin
	options, file, err := jmapOptionsAndLocation(req)
	if err != nil {
		return status.Errorf(codes.Internal, "can't generate options/heap dump location: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(file)) // clean up

	run, err := util.RunCommand(stream.Context(), cmdName, options)
	if err != nil {
		return err
	}

	// Don't care about stderr output since jmap produces some debug that way.
	if err := run.Error; err != nil {
		return status.Errorf(codes.Internal, "command exited with error: %v\n%s", err, util.TrimString(run.Stderr.String()))
	}

	f, err := os.Open(file)
	if err != nil {
		return status.Errorf(codes.Internal, "can't open %s for processing: %v", file, err)
	}
	defer f.Close()

	b := make([]byte, util.StreamingChunkSize)
	for {
		n, err := f.Read(b)
		// We're done on EOF.
		if err == io.EOF {
			break
		}

		if err != nil {
			return status.Errorf(codes.Internal, "can't read file %s: %v", file, err)
		}

		// Only send over the number of bytes we actually read or
		// else we'll send over garbage in the last packet potentially.
		if err := stream.Send(&pb.GetJavaHeapDumpReply{Data: b[:n]}); err != nil {
			return status.Errorf(codes.Internal, "can't send on stream: %v", err)
		}
	}
	return nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterProcessServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
