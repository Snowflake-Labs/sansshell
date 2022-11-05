/* Copyright (c) 2022 Snowflake Inc. All rights reserved.

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
	"fmt"
	"net"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/dns"
)

var (
	// Create package level resolver such that it can be replaced during testing
	defaultResolver = func() Resolver {
		return resolver{}
	}
)

// Server is used to implement the gRPC Server
type server struct{}

type Resolver interface {
	LookupIP(ctx context.Context, network, hostname string) ([]net.IP, error)
}

type resolver struct{}

func (resolver) LookupIP(ctx context.Context, network, hostname string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, network, hostname)
}

func (s *server) Lookup(ctx context.Context, req *pb.LookupRequest) (*pb.LookupReply, error) {
	logger := logr.FromContextOrDiscard(ctx)
	hostname := req.GetHostname()

	logger.Info("dns request", "hostname", hostname)
	// TODO(elsesiy): We only care about ipv4 for now but we could allow clients to explicitly specify opts such as network, prefer go resolver, etc.
	ips, err := defaultResolver().LookupIP(ctx, "ip4", hostname)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lookup %q", hostname)
	}

	var outString string
	for _, ip := range ips {
		outString += fmt.Sprintf("%s\n", ip.String())
	}

	reply := &pb.LookupReply{
		Result: []byte(outString),
	}

	return reply, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterLookupServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
