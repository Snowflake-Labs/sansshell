/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

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

// Package server implements the sansshell 'Logging' service.
package server

import (
	"context"
	"sync"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/sansshell"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server is used to implement the gRPC Server
type Server struct {
	mu      sync.RWMutex
	lastVal int32
}

// SetVerbosity sets the logging level and returns the last value before this was called.
func (s *Server) SetVerbosity(ctx context.Context, req *pb.SetVerbosityRequest) (*pb.VerbosityReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logger := logr.FromContextOrDiscard(ctx)
	old := int32(stdr.SetVerbosity(int(req.Level)))
	logger.Info("set-verbosity", "new level", req.Level, "old level", old)
	reply := &pb.VerbosityReply{
		Level: old,
	}
	s.lastVal = req.Level
	return reply, nil
}

// GetVerbosity returns the last set value (or 0 if it's never been set).
func (s *Server) GetVerbosity(ctx context.Context, req *emptypb.Empty) (*pb.VerbosityReply, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &pb.VerbosityReply{Level: s.lastVal}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *Server) Register(gs *grpc.Server) {
	pb.RegisterLoggingServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&Server{})
}
