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

package client

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/sansshell"
)

func GetSansshellVersion(ctx context.Context, conn *proxy.Conn) (string, error) {
	if len(conn.Targets) != 1 {
		return "", errors.New("GetSansshellVersion support only single target")
	}

	client := pb.NewStateClient(conn)

	resp, err := client.Version(ctx, &emptypb.Empty{})
	if err != nil {
		return "", fmt.Errorf("Could not get proxy version: %v\n", err)
	}

	return resp.GetVersion(), nil
}
