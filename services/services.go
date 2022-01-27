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

// Package services provides functions to register and list all the
// services contained in a sansshell gRPC server.
package services

import (
	"sync"

	"google.golang.org/grpc"
)

var (
	mu          sync.RWMutex
	rpcServices []SansShellRPCService
)

// SansShellRPCService provides an interface for services to implement
// to make them registerable in this package.
type SansShellRPCService interface {
	Register(*grpc.Server)
}

// RegisterSansShellService provides a mechanism for imported modules to
// register themselves with a gRPC server.
func RegisterSansShellService(s SansShellRPCService) {
	mu.Lock()
	defer mu.Unlock()
	rpcServices = append(rpcServices, s)
}

// ListServices returns the list of registered services.
func ListServices() []SansShellRPCService {
	mu.RLock()
	defer mu.RUnlock()
	return rpcServices
}
