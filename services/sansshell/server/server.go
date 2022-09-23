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
	"sync"

	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/sansshell"
)

// Server is used to implement the gRPC Server
type Server struct {
	mu      sync.RWMutex
	lastVal int32
}

// Register is called to expose this handler to the gRPC server
func (s *Server) Register(gs *grpc.Server) {
	pb.RegisterLoggingServer(gs, s)
	pb.RegisterStateServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&Server{})
}
