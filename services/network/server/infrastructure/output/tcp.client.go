/*
Copyright (c) 2019 Snowflake Inc. All rights reserved.

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

package output

import (
	"context"
	"fmt"
	app "github.com/Snowflake-Labs/sansshell/services/network/server/application"
	"net"
	"strconv"
	"strings"
	"time"
)

// TCPClient is implementation of [github.com/Snowflake-Labs/sansshell/services/network/server/application.TCPClientPort] interface
type TCPClient struct {
}

// CheckConnectivity is used to check tcp connectivity from remote machine to specified server
func (p *TCPClient) CheckConnectivity(ctx context.Context, hostname string, port uint8, timeout time.Duration) (*app.TCPConnectivityCheckResult, error) {
	hostToCheck := net.JoinHostPort(hostname, strconv.Itoa(int(port)))

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", hostToCheck)
	if err != nil {
		var failReason string
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			failReason = "Connection timed out"
		} else if opErr, ok := err.(*net.OpError); ok && opErr.Op == "dial" {
			if strings.HasSuffix(opErr.Err.Error(), "no such host") {
				failReason = "No such host"
			} else if strings.HasSuffix(opErr.Err.Error(), "connection refused") {
				failReason = "Connection refused"
			}
		}

		if failReason == "" {
			return nil, fmt.Errorf("unexpected error: %s", err.Error())
		}

		return &app.TCPConnectivityCheckResult{
			IsOk:       false,
			FailReason: &failReason,
		}, nil
	}

	defer conn.Close()
	return &app.TCPConnectivityCheckResult{
		IsOk: true,
	}, nil
}
