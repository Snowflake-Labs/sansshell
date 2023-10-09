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

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/proto"
)

// RPCAuthInput is used as policy input to validate Sansshell RPCs
// NOTE: RPCAuthInputForLogging must be updated when this changes.
type RPCAuthInput struct {
	// The GRPC method name, as '/Package.Service/Method'
	Method string `json:"method"`

	// The request protocol buffer message
	Message proto.Message `json:"message"`

	// The message type as 'Package.Message'
	MessageType string `json:"type"`

	// Raw grpc metdata associated with this call.
	Metadata metadata.MD `json:"metadata"`

	// Information about the calling peer, if available.
	Peer *PeerAuthInput `json:"peer"`

	// Information about the host serving the RPC.
	Host *HostAuthInput `json:"host"`

	// Information about the environment in which the policy evaluation is
	// happening.
	Environment *EnvironmentInput `json:"environment"`

	// Implementation specific extensions.
	Extensions json.RawMessage `json:"extensions"`
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
		out.Message = req
	}
	out.Peer = PeerInputFromContext(ctx)
	return out, nil
}

// PeerInputFromContext populates peer information from the supplied
// context, if available.
func PeerInputFromContext(ctx context.Context) *PeerAuthInput {
	out := &PeerAuthInput{}
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil
	}
	out.Net = NetInputFromAddr(p.Addr)
	out.Cert = CertInputFrom(p.AuthInfo)
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
