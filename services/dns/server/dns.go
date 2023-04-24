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
	"net"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/dns"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
)

// Metrics
var (
	dnsLookupFailureCounter = metrics.MetricDefinition{Name: "actions_dns_lookup_failure",
		Description: "number of failures when performing dns.Lookup"}
)

var (
	// Create package level resolver such that it can be replaced during testing
	resolver = net.DefaultResolver.LookupIP
)

// Server is used to implement the gRPC Server
type server struct{}

func (s *server) Lookup(ctx context.Context, req *pb.LookupRequest) (*pb.LookupReply, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	hostname := req.GetHostname()

	logger.Info("dns request", "hostname", hostname)
	// TODO(elsesiy): We only care about ipv4 for now but we could allow clients to explicitly specify opts such as network, prefer go resolver, etc.
	ips, err := resolver(ctx, "ip4", hostname)
	if err != nil {
		errCounter := recorder.Counter(ctx, dnsLookupFailureCounter, 1)
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+dnsLookupFailureCounter.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to lookup %q", hostname)
	}

	reply := &pb.LookupReply{}
	for _, ip := range ips {
		reply.Result = append(reply.Result, ip.String())
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
