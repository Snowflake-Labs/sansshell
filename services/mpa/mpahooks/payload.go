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

package mpahooks

import (
	"context"
	"sync"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type skipMPAKey struct{}

// PayloadProviderInput describes the inner RPC being stored as an MPA request.
// Exactly one of Conn or Proxy should be set.
type PayloadProviderInput struct {
	Method string
	Req    proto.Message
	Conn   *grpc.ClientConn
	Proxy  *proxy.Conn
}

// PayloadProvider computes an optional custom_payload for a new MPA request.
// Integrators register a provider to attach deployment-specific metadata.
type PayloadProvider func(ctx context.Context, in PayloadProviderInput) (*anypb.Any, error)

var (
	payloadProviderMu sync.RWMutex
	payloadProvider   PayloadProvider
)

// RegisterPayloadProvider registers a callback that supplies custom_payload for
// new MPA Store requests. Pass nil to unregister.
func RegisterPayloadProvider(p PayloadProvider) {
	payloadProviderMu.Lock()
	defer payloadProviderMu.Unlock()
	payloadProvider = p
}

func getPayloadProvider() PayloadProvider {
	payloadProviderMu.RLock()
	defer payloadProviderMu.RUnlock()
	return payloadProvider
}

// WithSkipMPA marks a context so MPA client interceptors pass the RPC through
// without creating or waiting on an approval.
func WithSkipMPA(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipMPAKey{}, true)
}

func shouldSkipMPA(ctx context.Context) bool {
	v, ok := ctx.Value(skipMPAKey{}).(bool)
	return ok && v
}

func buildCustomPayload(ctx context.Context, method string, req proto.Message, conn *grpc.ClientConn, proxyConn *proxy.Conn) (*anypb.Any, error) {
	provider := getPayloadProvider()
	if provider == nil {
		return nil, nil
	}
	return provider(ctx, PayloadProviderInput{
		Method: method,
		Req:    req,
		Conn:   conn,
		Proxy:  proxyConn,
	})
}
