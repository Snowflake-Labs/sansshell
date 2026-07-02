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

	pb "github.com/Snowflake-Labs/sansshell/services/network"
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
			result, err := client.CheckConnectivity(context.Background(), test.host, uint32(test.port), 1*time.Second, 0)

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
		result, err := client.CheckConnectivity(context.Background(), notExistedHost, 20, 1*time.Second, 0)

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

	t.Run("It should succeed on repeated checks with the same source port due to SO_REUSEADDR", func(t *testing.T) {
		// ARRANGE
		// Reserve an ephemeral port then release it so we have a known free source port.
		tmp, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to find a free source port: %s", err.Error())
		}
		sourcePort := uint32(tmp.Addr().(*net.TCPAddr).Port)
		tmp.Close()

		client := &TCPClient{}

		// ACT — first check
		result, err := client.CheckConnectivity(context.Background(), localhost, uint32(port), 1*time.Second, sourcePort)

		// ASSERT
		if err != nil {
			t.Fatalf("Unexpected error on first check: %s", err.Error())
		}
		if !result.IsOk {
			t.Fatalf("Expected first check to succeed, got fail reason: %s", result.FailReason)
		}

		// ACT — second check immediately after, same source port.
		// Without SO_REUSEADDR the port would linger in TIME_WAIT and this would
		// fail with SOURCE_PORT_IN_USE.
		result, err = client.CheckConnectivity(context.Background(), localhost, uint32(port), 1*time.Second, sourcePort)

		// ASSERT
		if err != nil {
			t.Fatalf("Unexpected error on second check: %s", err.Error())
		}
		if !result.IsOk {
			t.Fatalf("Expected second check to succeed (SO_REUSEADDR should bypass TIME_WAIT), got fail reason: %s", result.FailReason)
		}
	})

	t.Run("It should return SOURCE_PORT_IN_USE when source port is already bound", func(t *testing.T) {
		// ARRANGE
		// Bind a listener on a local port so that same port cannot be used as source.
		blocker, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to start blocker listener: %s", err.Error())
		}
		defer blocker.Close()
		blockedPort := uint32(blocker.Addr().(*net.TCPAddr).Port)

		client := &TCPClient{}

		// ACT
		result, err := client.CheckConnectivity(context.Background(), localhost, uint32(port), 1*time.Second, blockedPort)

		// ASSERT
		if err != nil {
			t.Errorf("Unexpected error: %s", err.Error())
			return
		}

		if result.IsOk {
			t.Errorf("Expected ok=false but got true")
			return
		}

		if result.FailReason == nil {
			t.Errorf("Expected a fail reason but got nil")
			return
		}

		// NOTE: EADDRINUSE fires when the source addr is already in use.
		// On Linux the bind to the local address happens before the connect,
		// so the error surfaces as SOURCE_PORT_IN_USE.
		if *result.FailReason != pb.TCPCheckFailureReason_SOURCE_PORT_IN_USE {
			t.Errorf("Expected SOURCE_PORT_IN_USE but got %s", result.FailReason)
		}
	})
}
