package process

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative process.proto

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/Snowflake-Labs/sansshell/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var psBin = flag.String("ps_bin", "/usr/bin/ps", "Location of the ps command")

// Flags passed to ps -e -o <flags> to get the output needed to implement List.
var psOptions = []string{
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

var psFlags = []string{
	"--noheader",
	"-e",
	"-o",
	strings.Join(psOptions, ","),
}

// server is used to implement the gRPC server
type server struct {
}

func (s *server) List(ctx context.Context, req *ListRequest) (*ListReply, error) {
	cmdName := *psBin
	options := psFlags

	// We gather all the processes up and then filter by pid if needed at the end.
	cmd := exec.CommandContext(ctx, cmdName, options...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := cmd.Start(); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// TODO(jchacon): We're assuming on any error we don't have unbounded output.
	// Probably should just clip instead.
	errBuf, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if len(errBuf) != 0 {
		return nil, status.Error(codes.Internal, fmt.Sprintf("unexpected error output:\n%s", string(errBuf)))
	}

	scanner := bufio.NewScanner(stdout)

	// Parse entries into a map so we can filter by pid later if needed.
	entries := make(map[int64]*ProcessEntry)
	for scanner.Scan() {
		// We should get back exactly the same amount of fields as we asked but
		// cmd can have spaces so stop before it.
		text := scanner.Text()
		fields := strings.SplitN(text, " ", len(psOptions)-1)
		if len(fields) != len(psOptions) {
			return nil, status.Error(codes.Internal, fmt.Sprintf("got wrong field count for line %q", text))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("parsing error:\n%v", err))
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
