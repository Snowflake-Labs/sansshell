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

package application

import (
	"context"
	"errors"
	"fmt"
	pb "github.com/Snowflake-Labs/sansshell/services/network"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type NetworkClientProxyMock interface {
	TCPCheckOneMany(ctx context.Context, in *pb.TCPCheckRequest, opts ...grpc.CallOption) (<-chan *pb.TCPCheckManyResponse, error)
	TCPCheck(ctx context.Context, in *pb.TCPCheckRequest, opts ...grpc.CallOption) (*pb.TCPCheckReply, error)
}

// NewNetworkClientProxyMock creates a new NetworkClientProxyMock
// - availableHostsPorts is a list of <host>:<port> what is available on network
// - unexpectedErrorHosts is a list of hosts for what we have an unexpected error
func NewNetworkClientProxyMock(targetHosts []string, availableHostsPorts []string, unexpectedErrorHosts []string) NetworkClientProxyMock {
	return &networkClientProxyMock{
		TargetHosts:          targetHosts,
		AvailableHostsPorts:  availableHostsPorts,
		UnexpectedErrorHosts: unexpectedErrorHosts,
	}
}

type networkClientProxyMock struct {
	TargetHosts []string
	// <host>:<port> what is available on network
	AvailableHostsPorts []string
	// Hosts for what we have an unexpected error
	UnexpectedErrorHosts []string
}

func (n *networkClientProxyMock) TCPCheckOneMany(ctx context.Context, in *pb.TCPCheckRequest, opts ...grpc.CallOption) (<-chan *pb.TCPCheckManyResponse, error) {
	isUnexpectedError := false
	for _, host := range n.UnexpectedErrorHosts {
		if host == in.Hostname {
			isUnexpectedError = true
			break
		}
	}
	if isUnexpectedError {
		return nil, status.Errorf(status.Code(errors.New("some unexpected error")), "Some unexpected error")
	}

	responses := make(chan *pb.TCPCheckManyResponse)
	go func() {
		for i, target := range n.TargetHosts {
			resp, err := n.TCPCheck(ctx, &pb.TCPCheckRequest{
				Hostname: in.Hostname,
				Port:     in.Port,
				Timeout:  in.Timeout,
			}, opts...)
			responses <- &pb.TCPCheckManyResponse{
				Target: target,
				Index:  i,
				Resp:   resp,
				Error:  err,
			}
		}
		close(responses)
	}()

	return responses, nil
}

func (n *networkClientProxyMock) TCPCheck(ctx context.Context, in *pb.TCPCheckRequest, opts ...grpc.CallOption) (*pb.TCPCheckReply, error) {
	isUnexpectedError := false
	for _, host := range n.UnexpectedErrorHosts {
		if host == in.Hostname {
			isUnexpectedError = true
			break
		}
	}
	if isUnexpectedError {
		return nil, status.Errorf(status.Code(errors.New("some unexpected error")), "Some unexpected error")
	}

	isAvailable := false

	tmp := pb.TCPCheckFailureReason_CONNECTION_REFUSED
	failReason := &tmp

	for _, hostPort := range n.AvailableHostsPorts {
		if hostPort == fmt.Sprintf("%s:%d", in.Hostname, in.Port) {
			isAvailable = true
			failReason = nil
			break
		}
	}

	return &pb.TCPCheckReply{
		Ok:         isAvailable,
		FailReason: failReason,
	}, nil
}
