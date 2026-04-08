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

package server

import (
	"context"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
)

const tracerName = "sansshell-proxy"

var (
	tracerMu      sync.RWMutex
	currentTracer trace.Tracer
)

func init() { currentTracer = otel.Tracer(tracerName) }

func getTracer() trace.Tracer {
	tracerMu.RLock()
	t := currentTracer
	tracerMu.RUnlock()
	return t
}

// setTracer replaces the active tracer and returns a function that restores
// the previous one. Safe for concurrent use.
func setTracer(t trace.Tracer) (restore func()) {
	tracerMu.Lock()
	prev := currentTracer
	currentTracer = t
	tracerMu.Unlock()
	return func() {
		tracerMu.Lock()
		currentTracer = prev
		tracerMu.Unlock()
	}
}

// Span attribute keys for caller identity.
const (
	attrCallerPrincipal  attribute.Key = "sansshell.caller.principal"
	attrCallerGroups     attribute.Key = "sansshell.caller.groups"
	attrCallerAddress    attribute.Key = "sansshell.caller.address"
	attrCallerCertCN     attribute.Key = "sansshell.caller.cert.cn"
	attrCallerCertSPIFFE attribute.Key = "sansshell.caller.cert.spiffe_id"
)

// Span attribute keys for target stream information.
const (
	attrTargetAddress          attribute.Key = "sansshell.target.address"
	attrTargetMethod           attribute.Key = "sansshell.target.method"
	attrTargetStreamID         attribute.Key = "sansshell.target.stream_id"
	attrTargetStreamType       attribute.Key = "sansshell.target.stream_type"
	attrTargetDialTimeoutMs    attribute.Key = "sansshell.target.dial_timeout_ms"
	attrTargetAuthzDryRun      attribute.Key = "sansshell.target.authz_dry_run"
	attrTargetProxiedPrincipal attribute.Key = "sansshell.target.proxied_principal"
)

// Span attribute keys for aggregate proxy-level information.
const (
	attrProxyTargetCount   attribute.Key = "sansshell.proxy.target_count"
	attrProxyJustification attribute.Key = "sansshell.proxy.justification"
)

// Span attribute keys for authz events.
const (
	attrAuthzResult attribute.Key = "sansshell.authz.result"
	attrAuthzMethod attribute.Key = "sansshell.authz.method"
)

// Span attribute key for stream finish status.
const attrGRPCStatusCode attribute.Key = "grpc.status_code"

func streamType(clientStreams, serverStreams bool) string {
	switch {
	case clientStreams && serverStreams:
		return "bidi"
	case clientStreams:
		return "client_stream"
	case serverStreams:
		return "server_stream"
	default:
		return "unary"
	}
}

func callerAttrsFromPeer(peer *rpcauth.PeerAuthInput) []attribute.KeyValue {
	if peer == nil {
		return nil
	}
	var attrs []attribute.KeyValue
	if peer.Net != nil && peer.Net.Address != "" {
		attrs = append(attrs, attrCallerAddress.String(peer.Net.Address))
	}
	if peer.Principal != nil {
		if peer.Principal.ID != "" {
			attrs = append(attrs, attrCallerPrincipal.String(peer.Principal.ID))
		}
		if len(peer.Principal.Groups) > 0 {
			attrs = append(attrs, attrCallerGroups.String(strings.Join(peer.Principal.Groups, ",")))
		}
	}
	if peer.Cert != nil {
		if peer.Cert.Subject.CommonName != "" {
			attrs = append(attrs, attrCallerCertCN.String(peer.Cert.Subject.CommonName))
		}
		if peer.Cert.SPIFFEID != "" {
			attrs = append(attrs, attrCallerCertSPIFFE.String(peer.Cert.SPIFFEID))
		}
	}
	return attrs
}

// enrichRootSpan sets caller identity and justification attributes on the
// current span in ctx. It is called from dispatch once peer info is available.
func enrichRootSpan(ctx context.Context, peer *rpcauth.PeerAuthInput) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	if attrs := callerAttrsFromPeer(peer); len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(rpcauth.ReqJustKey); len(vals) > 0 {
			span.SetAttributes(attrProxyJustification.String(vals[0]))
		}
	}
}
