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
	pb "github.com/Snowflake-Labs/sansshell/services/network"
	"time"
)

type TCPConnectivityCheckResult struct {
	IsOk       bool
	FailReason *pb.TCPCheckFailureReason
}

type TCPClientPort interface {
	CheckConnectivity(ctx context.Context, hostname string, port uint8, timeout time.Duration) (*TCPConnectivityCheckResult, error)
}

type tcpCheckUsecase struct {
	tcpClientPort TCPClientPort
}

type TCPCheckUsecase interface {
	Run(ctx context.Context, hostname string, port uint8, timeout time.Duration) (*TCPConnectivityCheckResult, error)
}

func (t *tcpCheckUsecase) Run(ctx context.Context, hostname string, port uint8, timeout time.Duration) (*TCPConnectivityCheckResult, error) {
	result, err := t.tcpClientPort.CheckConnectivity(ctx, hostname, port, timeout)
	return result, err
}

func NewTCPCheckUsecase(client TCPClientPort) TCPCheckUsecase {
	return &tcpCheckUsecase{
		tcpClientPort: client,
	}
}
