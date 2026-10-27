/* Copyright (c) 2026 Snowflake Inc. All rights reserved.

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

package server

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func dialIdentified(t *testing.T, dial func(context.Context, string) (net.Conn, error)) identifiedTarget {
	t.Helper()
	conn, err := NewCredentialsDialer(insecure.NewCredentials(), grpc.WithContextDialer(dial)).DialContext(context.Background(), "target")
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn.(identifiedTarget)
}

func TestTargetPeerReportsConnectionError(t *testing.T) {
	conn := dialIdentified(t, func(context.Context, string) (net.Conn, error) {
		return nil, errors.New("no route to target")
	})

	_, err := conn.targetPeer(context.Background())
	if status.Code(err) != codes.Unavailable || !strings.Contains(err.Error(), "no route to target") {
		t.Fatalf("got %v, want Unavailable with the dial error", err)
	}
}

func TestTargetPeerCancelled(t *testing.T) {
	conn := dialIdentified(t, func(ctx context.Context, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := conn.targetPeer(ctx)
	if status.Code(err) != codes.Canceled {
		t.Fatalf("got %v, want Canceled", err)
	}
}
