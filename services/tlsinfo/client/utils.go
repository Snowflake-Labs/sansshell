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

package client

import (
	"context"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/tlsinfo"
)

type TLSInfoRequest struct {
	ServerAddress      string // hostname:port
	ServerName         string
	InsecureSkipVerify bool
}

func GetTLSInfo(ctx context.Context, conn *proxy.Conn, request TLSInfoRequest) ([]*pb.GetTLSCertificateManyResponse, error) {
	proxy := pb.NewTLSInfoClientProxy(conn)
	requestProto := &pb.TLSCertificateRequest{
		ServerAddress:      request.ServerAddress,
		InsecureSkipVerify: request.InsecureSkipVerify,
		ServerName:         request.ServerName,
	}

	respChan, err := proxy.GetTLSCertificateOneMany(ctx, requestProto)
	if err != nil {
		return nil, err
	}
	ret := make([]*pb.GetTLSCertificateManyResponse, len(conn.Targets))
	for r := range respChan {
		if r.Error != nil {
			return nil, fmt.Errorf("target %s (index %d) returned error - %v", r.Target, r.Index, r.Error)
		}
		ret[r.Index] = &pb.GetTLSCertificateManyResponse{
			Target: r.Target,
			Index:  r.Index,
			Resp:   r.Resp,
		}
	}

	return ret, nil
}
