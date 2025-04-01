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

package rpcauth

import (
	"context"
	"crypto/x509/pkix"
	"encoding/json"
	"net"
	"reflect"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// RPCAuthInput is used as policy input to validate Sansshell RPCs
type RPCAuthInput struct {
	// The GRPC method name, as '/Package.Service/Method'
	Method string `json:"method"`

	// The request protocol buffer, serialized as JSON.
	Message json.RawMessage `json:"message"`

	// The message type as 'Package.Message'
	MessageType string `json:"type"`

	// Raw grpc metdata associated with this call.
	Metadata metadata.MD `json:"metadata"`

	// Information about the calling peer, if available.
	Peer *PeerAuthInput `json:"peer"`

	// Information about the host serving the RPC.
	Host *HostAuthInput `json:"host"`

	// Information about approvers when using multi-party authentication.
	Approvers []*PrincipalAuthInput `json:"approvers"`

	// TODO: commentary.
	ApprovedMethodWideMpa bool `json:"approved-method-wide-mpa"`

	// Information about the environment in which the policy evaluation is
	// happening.
	Environment *EnvironmentInput `json:"environment"`

	// Implementation specific extensions.
	Extensions json.RawMessage `json:"extensions"`

	// TargetConn is a connection to the target under evaluation. It is only non-nil when
	// the policy evaluation is being performed by some entity other than the host and
	// can be used in rpcauth hooks to gather information by making RPC calls to the
	// host.
	// TargetConn is not exposed to policy evaluation.
	TargetConn grpc.ClientConnInterface `json:"-"`
}

// EnvironmentInput contains information about the environment in which the policy evaluation is
// happening.
type EnvironmentInput struct {
	// True if the policy evaluation is being performed by some entity other than the host
	// (for example, policy checks performed by an authorizing proxy or other agent)
	NonHostPolicyCheck bool `json:"non_host_policy_check"`
}

// PeerAuthInput contains policy-relevant information about an RPC peer.
type PeerAuthInput struct {
	// Network information about the peer
	Net *NetAuthInput `json:"net"`

	// Unix peer credentials if peer connects via Unix socket, nil otherwise
	Unix *UnixAuthInput `json:"unix"`

	// Information about the certificate presented by the peer, if any
	Cert *CertAuthInput `json:"cert"`

	// Information about the principal associated with the peer, if any
	Principal *PrincipalAuthInput `json:"principal"`
}

// NetAuthInput contains policy-relevant information related to a network endpoint
type NetAuthInput struct {
	// The 'network' as returned by net.Addr.Network() (e.g. "tcp", "udp")
	Network string `json:"network"`

	// The address string, as returned by net.Addr.String(), with port (if any) removed
	Address string `json:"address"`

	// The port, as parsed from net.Addr.String(), if present
	Port string `json:"port"`
}

// UnixAuthInput contains information about a Unix socket peer.
type UnixAuthInput struct {
	// The user ID of the peer.
	Uid int `json:"uid"`

	// The username of the peer, or the stringified UID if user is not known.
	UserName string `json:"username"`

	// The group IDs (primary and supplementary) of the peer.
	Gids []int `json:"gids"`

	// The group names of the peer. If not available, the stringified IDs is used.
	GroupNames []string `json:"groupnames"`
}

// HostAuthInput contains policy-relevant information about the system receiving
// an RPC
type HostAuthInput struct {
	// The host address
	Net *NetAuthInput `json:"net"`

	// Information about the certificate served by the host, if any
	Cert *CertAuthInput `json:"cert"`

	// Information about the principal associated with the host, if any
	Principal *PrincipalAuthInput `json:"principal"`
}

// CertAuthInput contains policy-relevant information derived from a certificate
type CertAuthInput struct {
	// The certificate subject
	Subject pkix.Name `json:"subject"`

	// The certificate issuer
	Issuer pkix.Name `json:"issuer"`

	// DNS names, from SubjectAlternativeName
	DNSNames []string `json:"dnsnames"`

	// The raw SPIFFE identifier, if present
	SPIFFEID string `json:"spiffeid"`
}

// PrincipalAuthInput contains policy-relevant information about the principal
// associated with an operation.
type PrincipalAuthInput struct {
	// The principal identifier (e.g. a username or service role)
	ID string `json:"id"`

	// Auxilliary groups associated with this principal.
	Groups []string `json:"groups"`
}

// NewRPCAuthInput creates RpcAuthInput for the supplied method and request, deriving
// other information (if available) from the context.
func NewRPCAuthInput(ctx context.Context, method string, req proto.Message) (*RPCAuthInput, error) {
	out := &RPCAuthInput{
		Method: method,
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		out.Metadata = md
	}

	if req != nil {
		out.MessageType = string(proto.MessageName(req))
		marshaled, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(req)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "error marshalling request for auth: %v", err)
		}
		out.Message = json.RawMessage(marshaled)
	}
	out.Peer = PeerInputFromContext(ctx)
	return out, nil
}

type peerInfoKey struct{}

// AddPeerToContext adds a PeerAuthInput to the context. This is typically
// added by the rpcauth grpc interceptors.
func AddPeerToContext(ctx context.Context, p *PeerAuthInput) context.Context {
	if p == nil {
		return ctx
	}
	return context.WithValue(ctx, peerInfoKey{}, p)
}

// PeerInputFromContext populates peer information from the supplied
// context, if available.
func PeerInputFromContext(ctx context.Context) *PeerAuthInput {
	cached, _ := ctx.Value(peerInfoKey{}).(*PeerAuthInput)

	out := &PeerAuthInput{}
	p, ok := peer.FromContext(ctx)
	if !ok {
		// If there's no peer info, returned cached data so that invocations
		// of AddPeerToContext can work outside of RPC contexts.
		return cached
	}

	out.Net = NetInputFromAddr(p.Addr)
	out.Unix = UnixInputFrom(p.AuthInfo)
	out.Cert = CertInputFrom(p.AuthInfo)

	// If this runs after rpcauth hooks, we can return richer data that includes
	// information added by the hooks.
	// We need to compare cached data to peer info because we might be calling
	// PeerInputFromContext on the context of a client stream, which has a peer
	// of the target being called and may have the cached value from an earlier
	// server authorization.
	if cached != nil && cached.Principal != nil && reflect.DeepEqual(out.Net, cached.Net) {
		out.Principal = &PrincipalAuthInput{
			ID:     cached.Principal.ID,
			Groups: cached.Principal.Groups,
		}
	}
	return out
}

// NetInputFromAddr returns NetAuthInput from the supplied net.Addr
func NetInputFromAddr(addr net.Addr) *NetAuthInput {
	if addr == nil {
		return nil
	}
	out := &NetAuthInput{
		Network: addr.Network(),
		Address: addr.String(),
	}
	if host, port, err := net.SplitHostPort(addr.String()); err == nil {
		out.Address = host
		out.Port = port
	}
	return out
}

// UnixInputFrom returns UnixAuthInput from the supplied credentials, if available.
func UnixInputFrom(authInfo credentials.AuthInfo) *UnixAuthInput {
	if unixInfo, ok := authInfo.(UnixPeerAuthInfo); ok {
		return &UnixAuthInput{
			Uid:        unixInfo.Credentials.Uid,
			UserName:   unixInfo.Credentials.UserName,
			Gids:       unixInfo.Credentials.Gids,
			GroupNames: unixInfo.Credentials.GroupNames,
		}
	}
	return nil
}

// CertInputFrom populates certificate information from the supplied
// credentials, if available.
func CertInputFrom(authInfo credentials.AuthInfo) *CertAuthInput {
	if authInfo == nil {
		return nil
	}
	out := &CertAuthInput{}
	tlsInfo, ok := authInfo.(credentials.TLSInfo)
	if !ok {
		return out
	}
	if tlsInfo.SPIFFEID != nil {
		out.SPIFFEID = tlsInfo.SPIFFEID.String()
	}
	if len(tlsInfo.State.PeerCertificates) > 0 {
		// Element 0 is the 'leaf' cert, which is used to verify
		// the connection.
		cert := tlsInfo.State.PeerCertificates[0]
		out.Subject = cert.Subject
		out.Issuer = cert.Issuer
		out.DNSNames = cert.DNSNames
	}
	return out
}
