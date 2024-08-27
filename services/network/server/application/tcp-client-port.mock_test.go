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
	"time"
)

// TCPClientPortMock is a mock for [app.TCPClientPort]
type TCPClientPortMock interface {
	TCPClientPort
}

// NewTCPClientPortMock creates a new TCPClientPortMock
// - availableHostsPorts is a list of <host>:<port> what is available on network
// - unexpectedErrorHosts is a list of hosts for what we have an unexpected error
func NewTCPClientPortMock(availableHostsPorts []string, unexpectedErrorHosts []string) TCPClientPortMock {
	return &tcpClientPortMock{
		AvailableHostsPorts:  availableHostsPorts,
		UnexpectedErrorHosts: unexpectedErrorHosts,
	}
}

type tcpClientPortMock struct {
	// <host>:<port> what is available on network
	AvailableHostsPorts []string
	// Hosts for what we have an unexpected error
	UnexpectedErrorHosts []string
}

func (n *tcpClientPortMock) CheckConnectivity(ctx context.Context, hostname string, port uint32, timeout time.Duration) (*TCPConnectivityCheckResult, error) {
	isUnexpectedError := false
	for _, unexpectedErrorHost := range n.UnexpectedErrorHosts {
		if unexpectedErrorHost == hostname {
			isUnexpectedError = true
			break
		}
	}
	if isUnexpectedError {
		return nil, errors.New("some unexpected error")
	}

	isAvailable := false

	tmp := pb.TCPCheckFailureReason_CONNECTION_REFUSED
	failReason := &tmp

	for _, hostPort := range n.AvailableHostsPorts {
		if hostPort == fmt.Sprintf("%s:%d", hostname, port) {
			isAvailable = true
			failReason = nil
			break
		}
	}

	return &TCPConnectivityCheckResult{
		IsOk:       isAvailable,
		FailReason: failReason,
	}, nil
}
