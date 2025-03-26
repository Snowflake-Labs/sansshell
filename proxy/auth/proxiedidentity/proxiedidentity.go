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

// Package proxiedidentity provides a way to pass the identity of an end user
// through the SansShell proxy
package proxiedidentity

import (
	"context"
	"encoding/json"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// We don't expose the key so that people aren't tempted to use it directly.
// We intentionally prefix this with something other than sansshell- so that
// https://github.com/Snowflake-Labs/sansshell/blob/main/telemetry/telemetry.go
// doesn't accidentally send untrusted data in passAlongMetadata().
const reqProxiedIdentityKey = "proxied-sansshell-identity"

// ServerProxiedIdentityUnaryInterceptor is a no-op.
//
// Deprecated: This was formerly used to avoid unintentional proxying
func ServerProxiedIdentityUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		return handler(ctx, req)
	}
}

// AppendToMetadataInOutgoingContext includes the identity in the grpc metadata
// used in outgoing calls with the context.
func AppendToMetadataInOutgoingContext(ctx context.Context, p *rpcauth.PrincipalAuthInput) context.Context {
	b, err := json.Marshal(p)
	if err != nil {
		// There shouldn't be any possible value of PrincipalAuthInput that fails to marshal, so let's
		// return the original context so that the caller doesn't need to consider failures.
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, reqProxiedIdentityKey, string(b))
}

// FromContext returns the identity in ctx if it exists.
//
// This should ONLY be used if the caller is trusted to proxy requests. The
// best way to enforce this is to reject RPC requests that set `proxied-sansshell-identity`
// in the gRPC metadata when they come from callers other than a proxy.
//
// Failing to do this authz check can let any caller assert any proxied identity, which
// can let a caller take dangerous actions like approving their own MPA requests.
func FromContext(ctx context.Context) *rpcauth.PrincipalAuthInput {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	identity := md.Get(reqProxiedIdentityKey)
	if len(identity) != 1 {
		// No need to do anything more if there's no proxied identity
		return nil
	}

	parsed := new(rpcauth.PrincipalAuthInput)
	if err := json.Unmarshal([]byte(identity[0]), parsed); err != nil {
		return nil
	}
	return parsed
}

// ServerProxiedIdentityStreamInterceptor is a no-op.
//
// Deprecated: This was formerly used to avoid unintentional proxying
func ServerProxiedIdentityStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	}
}
