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

// Package server implements the sansshell 'httpoverrpc' service.
package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/httpoverrpc"
	sansshellserver "github.com/Snowflake-Labs/sansshell/services/sansshell/server"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Metrics
var (
	localhostFailureCounter = metrics.MetricDefinition{Name: "actions_httpoverrpc_localhost_failure",
		Description: "number of failures when performing HTTPOverRPC/Localhost"}
)

var (
	httpClient = &http.Client{}
)

// Server is used to implement the gRPC Server
type server struct{}

func (s *server) Host(ctx context.Context, req *pb.HostHTTPRequest) (*pb.HTTPReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	hostname := "localhost"
	if req.Hostname != "" {
		hostname = req.Hostname
	}
	if req.Protocol != "http" && req.Protocol != "https" {
		return nil, status.Errorf(codes.InvalidArgument, "currently request protocol can only be http or https")
	}

	url := fmt.Sprintf("%s://%s:%v%v", req.Protocol, hostname, req.Port, req.Request.RequestUri)
	httpReq, err := http.NewRequestWithContext(ctx, req.Request.Method, url, bytes.NewReader(req.Request.Body))
	if err != nil {
		recorder.CounterOrLog(ctx, localhostFailureCounter, 1)
		return nil, err
	}
	// Set a default user agent that can be overridden in the request.
	httpReq.Header["User-Agent"] = []string{"sansshell/" + sansshellserver.Version}
	for _, header := range req.Request.Headers {
		httpReq.Header[header.Key] = header.Values
	}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	var respHeaders []*pb.Header
	for k, v := range httpResp.Header {
		respHeaders = append(respHeaders, &pb.Header{Key: k, Values: v})
	}
	return &pb.HTTPReply{
		StatusCode: int32(httpResp.StatusCode),
		Headers:    respHeaders,
		Body:       body,
	}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterHTTPOverRPCServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
