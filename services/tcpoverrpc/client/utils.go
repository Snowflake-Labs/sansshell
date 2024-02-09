/*
Copyright (c) 2023 Snowflake Inc. All rights reserved.

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
package client

import (
	"context"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/tcpoverrpc"
)

func TCPValidateMany(ctx context.Context, conn *proxy.Conn, hostname string, port int32) ([]*pb.OkManyResponse, error) {
	c := pb.NewTCPOverRPCClientProxy(conn)

	respChan, err := c.OkOneMany(ctx, &pb.HostTCPRequest{Hostname: hostname, Port: port})
	if err != nil {
		return nil, err
	}
	results := make([]*pb.OkManyResponse, len(conn.Targets))
	for r := range respChan {
		results[r.Index] = r
	}

	return results, nil
}
