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
	"net"
	"os"
	"testing"
	"time"
)

const localhost = "localhost"
const notExistedHost = "127.50.50.50"

func startTCPServer() (net.Listener, int, error) {
	port := 8081
	var listener net.Listener
	for port < 9000 {
		var err error
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", localhost, port))

		if err != nil {
			port++
			continue
		}
		break
	}

	if listener == nil {
		return nil, 0, fmt.Errorf("failed to start TCP server")
	}

	go (func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	})()

	return listener, port, nil
}

func TestIntegrationTCPClient_CheckConnectivity(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test")
	}

	listener, port, err := startTCPServer()

	if err != nil {
		t.Fatalf("Failed to start TCP server: %s", err.Error())
	}
	defer listener.Close()

	tests := []struct {
		name           string
		port           int
		host           string
		expectedStatus bool
	}{
		{
			name:           "It should return ok, in case server listening",
			port:           port,
			host:           localhost,
			expectedStatus: true,
		},
		{
			name:           "It should return Not OK, in case server not exists",
			port:           port,
			host:           notExistedHost,
			expectedStatus: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			client := &TCPClient{}

			// ACT
			result, err := client.CheckConnectivity(context.Background(), test.host, uint32(test.port), 1*time.Second)

			// ASSERT
			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
				return
			}

			if result.IsOk != test.expectedStatus {
				t.Errorf("Expected \"%t\", but provided \"%t\", fail reason: %s", test.expectedStatus, result.IsOk, *result.FailReason)
			}
		})
	}

	t.Run("It should be fail reason, if status not OK", func(t *testing.T) {
		// ARRANGE
		client := &TCPClient{}

		// ACT
		result, err := client.CheckConnectivity(context.Background(), notExistedHost, 20, 1*time.Second)

		// ASSERT
		if err != nil {
			t.Errorf("Unexpected error: %s", err.Error())
			return
		}

		if result.IsOk == true {
			t.Errorf("Status should be false")
			return
		}

		if result.FailReason == nil {
			t.Errorf("Fail reason should be not nil")
		}
	})
}
