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
	"io"
	"testing"
	"time"

	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
)

func TestTargetStreamSetEmpty(t *testing.T) {
	set := NewTargetStreamSet(nil, nil, nil)
	if !set.Empty() {
		t.Error("a new set is not empty")
	}
	set.streams[1] = &TargetStream{}
	if set.Empty() {
		t.Error("a set holding a stream reports empty")
	}
	set.Remove(1)
	if !set.Empty() {
		t.Error("a set reports non-empty after its only stream was removed")
	}
}

// The RPC must end on its own once the client half-closes and the target streams have
// finished. It used to stay open until the client disconnected, so Recv blocked here forever.
func TestProxyEndsAfterClientCloseSend(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123")
	proxyStream := startTestProxy(ctx, t, testServerMap)

	streamID := testutil.MustStartStream(t, proxyStream, "foo:123", "/Testdata.TestService/TestUnary")
	req := testutil.PackStreamData(t, &tdpb.TestRequest{Input: "Foo"}, streamID)
	testutil.Exchange(t, proxyStream, req)

	if err := proxyStream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		for {
			if _, err := proxyStream.Recv(); err != nil {
				done <- err
				return
			}
		}
	}()

	select {
	case err := <-done:
		if err != io.EOF {
			t.Errorf("stream ended with %v, want io.EOF", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the proxy RPC did not end after the client half-closed")
	}
}
