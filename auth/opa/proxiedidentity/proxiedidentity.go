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
	"errors"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// We don't expose the key so that people aren't tempted to use it directly.
// We intentionally prefix this with something other than sansshell- so that
// https://github.com/Snowflake-Labs/sansshell/blob/main/telemetry/telemetry.go
// doesn't accidentally send untrusted data in passAlongMetadata().
const reqProxiedIdentityKey = "proxied-sansshell-identity"

// ServerProxiedIdentityUnaryInterceptor adds information about a proxied caller to the RPC context
// if the provided function returns true. Allow functions will typically pull out information on the
// caller's identity from the context with https://godoc.org/google.golang.org/grpc/peer to decide
// if the addition is allowed.
func ServerProxiedIdentityUnaryInterceptor(allow func(context.Context) bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		identity, ok := fromMetadataInContext(ctx)
		if !ok {
			// No need to do anything more if there's no proxied identity
			return handler(ctx, req)
		}
		if !allow(ctx) {
			return nil, errors.New("peer not allowed to proxy identities")
		}
		ctx = newContext(ctx, identity)
		return handler(ctx, req)
	}
}

type proxiedIdentityKey struct{}

// newContext creates a new context with the identity attached.
func newContext(ctx context.Context, p *rpcauth.PrincipalAuthInput) context.Context {
	return context.WithValue(ctx, proxiedIdentityKey{}, p)
}

// FromContext returns the identity in ctx if it exists. It will typically
// only exist if ServerProxiedIdentityUnaryInterceptor was used.
func FromContext(ctx context.Context) (p *rpcauth.PrincipalAuthInput, ok bool) {
	p, ok = ctx.Value(proxiedIdentityKey{}).(*rpcauth.PrincipalAuthInput)
	return
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

// fromMetadataInContext fetches the identity from the grpc metadata
// embedded within the context if it exists. If using this, ensure
// that the metadata comes from a trusted source.
func fromMetadataInContext(ctx context.Context) (p *rpcauth.PrincipalAuthInput, ok bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, false
	}
	identity := md.Get(reqProxiedIdentityKey)
	if len(identity) != 1 {
		// No need to do anything more if there's no proxied identity
		return nil, false
	}

	parsed := new(rpcauth.PrincipalAuthInput)
	if err := json.Unmarshal([]byte(identity[0]), parsed); err != nil {
		return nil, false
	}
	return parsed, true
}
