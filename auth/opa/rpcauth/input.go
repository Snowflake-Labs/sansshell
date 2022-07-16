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
	"fmt"
	"net"
	"reflect"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// RPCAuthInput is used as policy input to validate Sansshell RPCs
// NOTE: RPCAuthInputForLogging must be updated when this changes.
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

	// Implementation specific extensions.
	Extensions json.RawMessage `json:"extensions"`
}

// RPCAuthInputForLogging is a mirror struct for RPCAuthInput which
// translates the json.RawMessage fields into strings type wise.
// This allows logging as human readable values vs [xx, yy, zz] style
// NOTE: This must be updated when RPCAuthInput changes.
type RPCAuthInputForLogging struct {
	Method      string         `json:"method"`
	Message     string         `json:"message"`
	MessageType string         `json:"type"`
	Metadata    metadata.MD    `json:"metadata"`
	Peer        *PeerAuthInput `json:"peer"`
	Host        *HostAuthInput `json:"host"`
	Extensions  string         `json:"extensions,string"`
}

func (r RPCAuthInput) MarshalLog() interface{} {
	// Transform for logging by forcing the types
	// for raw JSON into string. Otherwise logr
	// will print them as strings of bytes. If we
	// cast to string for the whole object then
	// we end up with string escaping.
	return RPCAuthInputForLogging{
		Method:      r.Method,
		Message:     string(r.Message),
		MessageType: r.MessageType,
		Metadata:    r.Metadata,
		Peer:        r.Peer,
		Host:        r.Host,
		Extensions:  string(r.Extensions),
	}
}

func init() {
	// Sanity check we didn't get the 2 structs out of sync.
	r := reflect.TypeOf(RPCAuthInput{})
	rl := reflect.TypeOf(RPCAuthInputForLogging{})

	rFields := reflect.VisibleFields(r)
	rlFields := reflect.VisibleFields(rl)
	m := make(map[string]bool)
	ml := make(map[string]bool)
	for _, f := range rFields {
		m[f.Name] = true
	}
	for _, f := range rlFields {
		ml[f.Name] = true
	}
	if !reflect.DeepEqual(m, ml) {
		panic(fmt.Sprintf("fields from RPCAuthInput (%v) do not match fields from RPCAuthInputForLogging (%v)", m, ml))
	}
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
		marshaled, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(req)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "error marshalling request for auth: %v", err)
		}
		out.Message = json.RawMessage(marshaled)
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
		return out
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
