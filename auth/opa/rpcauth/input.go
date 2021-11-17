package rpcauth

import (
	"context"
	"crypto/x509/pkix"
	"encoding/json"
	"net"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// RpcAuthInput is used as policy input to validate Sansshell RPCs
type RpcAuthInput struct {
	// The GRPC method name, as '/Package.Service/Method'
	Method string `json:"method"`

	// The request protocol buffer, serialized as JSON
	Message json.RawMessage `json:"message"`

	// The message type as 'Package.Message'
	MessageType string `json:"type"`

	// Raw grpc metdata associated with this call.
	Metadata metadata.MD `json:"metadata"`

	// Information about the calling peer, if available
	Peer *PeerAuthInput `json:"peer"`

	// Information about the host serving the RPC.
	Host *HostAuthInput `json:"host"`
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

// NewRpcAuthInput creates RpcAuthInput for the supplied method and request, deriving
// other information (if available) from the context.
func NewRpcAuthInput(ctx context.Context, method string, req proto.Message) (*RpcAuthInput, error) {
	out := &RpcAuthInput{
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

// NetInputFrom returns NetAuthInput from the supplied net.Addr
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
