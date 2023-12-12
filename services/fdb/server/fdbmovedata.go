/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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
	"context"
	"fmt"
	"io"
	"math/rand"
	"os/exec"
	"sync"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	// fdb_move_orchestrator binary location
	// Will be checked into:
	// https://github.com/apple/foundationdb/tree/snowflake/release-71.3/bindings/python/fdb/fdb_move_orchestrator.py
	FDBMoveOrchestrator     string
	generateFDBMoveDataArgs = generateFDBMoveDataArgsImpl
)

const maxHistoryBytes = 1024 * 1024

// reader allows reading incoming data from a multiReader
type reader struct {
	next   chan []byte
	parent *multiReader
}

func (r *reader) Next(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b := <-r.next:
		return b, nil
	case <-r.parent.finished:
		return nil, r.parent.finalErr
	}
}

// multiReader continuously reads from the provided io.Reader and provides ways
// for multiple callers to read from the data it provides. It internally retains
// maxHistoryBytes of past data.
type multiReader struct {
	src      io.Reader
	history  []byte
	readers  []*reader
	finalErr error
	finished chan struct{}
	mu       sync.Mutex
}

func newMultiReader(src io.Reader) *multiReader {
	m := &multiReader{src: src, finished: make(chan struct{})}
	go m.backgroundRead()
	return m
}

func (m *multiReader) backgroundRead() {
	for {
		b := make([]byte, 512)
		n, err := m.src.Read(b)
		if err != nil {
			m.finalErr = err
			close(m.finished)
			return
		}
		b = b[:n]
		m.mu.Lock()
		for _, r := range m.readers {
			select {
			case r.next <- b:
			default:
			}
		}
		m.history = append(m.history, b...)
		if len(m.history) > maxHistoryBytes {
			m.history = m.history[maxHistoryBytes/2:]
		}
		m.mu.Unlock()
	}
}

// Reader returns the bytes read so far and a reader to use for subsequent reads.
func (m *multiReader) Reader() ([]byte, *reader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := &reader{next: make(chan []byte, 10), parent: m}
	m.readers = append(m.readers, r)
	return m.history, r
}

type moveOperation struct {
	req     *pb.FDBMoveDataCopyRequest
	stdout  *multiReader
	stderr  *multiReader
	done    chan struct{}
	exitErr *exec.ExitError
}

type fdbmovedata struct {
	mu         sync.Mutex
	operations map[int64]*moveOperation
}

func generateFDBMoveDataArgsImpl(req *pb.FDBMoveDataCopyRequest) ([]string, error) {
	command := []string{FDBMoveOrchestrator}

	var args []string
	args = append(args, "--cluster", req.ClusterFile)
	args = append(args, "--tenant-group", req.TenantGroup)
	args = append(args, "--src-name", req.SourceCluster)
	args = append(args, "--dst-name", req.DestinationCluster)
	args = append(args, "--num-procs", fmt.Sprintf("%d", req.NumProcs))

	command = append(command, args...)
	return command, nil
}

func (s *fdbmovedata) FDBMoveDataCopy(ctx context.Context, req *pb.FDBMoveDataCopyRequest) (*pb.FDBMoveDataCopyResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The sansshell server should only run one copy command at a time
	for id, o := range s.operations {
		if proto.Equal(o.req, req) {
			return &pb.FDBMoveDataCopyResponse{
				Id:       id,
				Existing: true,
			}, nil
		}
	}
	if len(s.operations) > 0 {
		return nil, status.Errorf(codes.Internal, "Copy command already running")
	}
	logger := logr.FromContextOrDiscard(ctx)
	id := rand.Int63()
	command, err := generateFDBMoveDataArgs(req)
	if err != nil {
		return nil, err
	}

	// Add env vars for python and python bindings
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = append(cmd.Environ(), "PYTHONPATH=/opt/rh/rh-python36/root/lib/python3.6/site-packages/")
	cmd.Env = append(cmd.Environ(), "PATH=/opt/rh/rh-python36/root/bin")
	cmd.Env = append(cmd.Environ(), "PYTHONUNBUFFERED=true")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	logger.Info("executing local command", "cmd", cmd.String())
	logger.Info("command running with id", "id", id)
	if err = cmd.Start(); err != nil {
		return nil, status.Errorf(codes.Internal, "error running fdbmovedata cmd (%+v): %v", command, err)
	}

	op := &moveOperation{
		req:    req,
		done:   make(chan struct{}),
		stdout: newMultiReader(stdout),
		stderr: newMultiReader(stderr),
	}
	s.operations[id] = op
	go func() {
		// Wait for output to be done, then check command status
		<-op.stdout.finished
		<-op.stderr.finished
		err := cmd.Wait()
		if exitErr, ok := err.(*exec.ExitError); ok {
			op.exitErr = exitErr
		}
		logger.Info("fdbmovedata command finished", "id", id, "err", err)
		close(op.done)
	}()

	resp := &pb.FDBMoveDataCopyResponse{
		Id:       id,
		Existing: false,
	}
	return resp, nil
}

func (s *fdbmovedata) FDBMoveDataWait(req *pb.FDBMoveDataWaitRequest, stream pb.FDBMove_FDBMoveDataWaitServer) error {
	ctx := stream.Context()
	logger := logr.FromContextOrDiscard(ctx)

	s.mu.Lock()
	op, found := s.operations[req.Id]
	var ids []int64
	for id := range s.operations {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	if !found {
		logger.Info("Provided ID and stored IDs do not match", "providedID", req.Id, "storedID", ids)
		return status.Errorf(codes.Internal, "Provided ID %d does not match stored IDs %v", req.Id, ids)
	}

	stdoutHistory, stdout := op.stdout.Reader()
	stderrHistory, stderr := op.stderr.Reader()
	if stdoutHistory != nil || stderrHistory != nil {
		if err := stream.Send(&pb.FDBMoveDataWaitResponse{Stdout: stdoutHistory, Stderr: stderrHistory}); err != nil {
			return err
		}
	}

	// Send output asynchronously
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			buf, err := stderr.Next(gCtx)
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("could not read stdout: %v", err)
			}
			if err := stream.Send(&pb.FDBMoveDataWaitResponse{Stderr: buf}); err != nil {
				return err
			}
		}
	})
	g.Go(func() error {
		for {
			buf, err := stdout.Next(ctx)
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("could not read stderr: %v", err)
			}
			if err := stream.Send(&pb.FDBMoveDataWaitResponse{Stdout: buf}); err != nil {
				return err
			}
		}
	})

	if err := g.Wait(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-op.done:
	}
	// clear the cmd to allow another call
	s.mu.Lock()
	delete(s.operations, req.Id)
	s.mu.Unlock()

	if op.exitErr != nil {
		return stream.Send(&pb.FDBMoveDataWaitResponse{RetCode: int32(op.exitErr.ExitCode())})
	}
	return nil
}

// Register is called to expose this handler to the gRPC server
func (s *fdbmovedata) Register(gs *grpc.Server) {
	pb.RegisterFDBMoveServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&fdbmovedata{operations: make(map[int64]*moveOperation)})
}
