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

package application

import (
	"context"
	"fmt"
	pb "github.com/Snowflake-Labs/sansshell/services/network"
	"google.golang.org/protobuf/types/known/durationpb"
	"time"
)

type tcpCheckUseCase struct {
	networkClient pb.NetworkClientProxy
}

// Run is used to check tcp connectivity from remote machine to specified server
// - ctx is context of the request
// - hostname is host of remote machine from which was checked tcp connectivity
// - port is port of remote machine from which was checked tcp connectivity
// - timeout is  seconds to wait for a response on remote machine from hostname
func (p *tcpCheckUseCase) Run(ctx context.Context, hostname string, port uint32, timeoutSeconds uint) (<-chan *pb.TCPCheckManyResponse, error) {
	req := &pb.TCPCheckRequest{
		Hostname: hostname,
		Port:     port,
		Timeout:  durationpb.New(time.Duration(timeoutSeconds) * time.Second),
	}

	var resp, err = p.networkClient.TCPCheckOneMany(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Unexpected error: %s\n", err.Error())
	}

	return resp, nil
}

type TCPCheckUseCase interface {
	Run(ctx context.Context, hostname string, port uint32, timeoutSeconds uint) (<-chan *pb.TCPCheckManyResponse, error)
}

func NewTCPCheckUseCase(networkClient pb.NetworkClientProxy) TCPCheckUseCase {
	return &tcpCheckUseCase{
		networkClient: networkClient,
	}
}
