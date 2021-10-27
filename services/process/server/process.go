package server

import (
	"bufio"
	"bytes"
	"context"
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

// The maximum we should allow stdout or stderr to be when sending back in an error string.
// grpc has limits on how large a returned error can be (generally 4-8k depending on language).
const MAX_BUF = 1024

func trimString(s string) string {
	if len(s) > MAX_BUF {
		s = s[:MAX_BUF]
	}
	return s
}

// A var so we can replace for testing.
var pstackOptions = func(req *pb.GetStacksRequest) []string {
	return []string{
		fmt.Sprintf("%d", req.Pid),
	}
}

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
	if *psStackBin == "" {
		return nil, status.Error(codes.Unimplemented, "not implemented")
	}

	if req.Pid <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pid must be non-zero and positive")
	}

	cmdName := *psStackBin
	options := pstackOptions(req)

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
	return nil, status.Error(codes.Unimplemented, "not implemented")
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
