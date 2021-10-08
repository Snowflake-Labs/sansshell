package server

// Based on this excellent Apache-licensed example by Stephan Renatus:
// https://github.com/open-policy-agent/opa/issues/1931

import (
	"context"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/open-policy-agent/opa/rego"
)

type OPA struct {
	policy rego.PreparedEvalQuery
}

// PeerInfo contains information about the sanshell calling
// peer that can be used for making policy decisions.
type PeerInfo struct {
	// The 'network' as returned by net.Addr.Network() (e.g. "tcp", "udp")
	Network string `json:"network"`
	// The address string, as returned by net.Addr.String()
	Address string `json:"address"`
	// Information about the certificate presented by the client, if any.
	CertInfo struct {
		// Structured names of the Subject and Issuer.
		Subject pkix.Name `json:"subject"`
		Issuer  pkix.Name `json:"issuer"`
		// Fields from the SubjectAlternativeName
		DNSNames []string `json:"dnsnames"`
		SPIFFEID string   `json:"spiffeid"`
	} `json:"cert"`
}

type input struct {
	Peer       *PeerInfo             `json:"peer"`
	Method     string                `json:"method"`
	ServerName string                `json:"servername"`
	Metadata   metadata.MD           `json:"metadata"`
	Message    json.RawMessage       `json:"message"`
	Type       protoreflect.FullName `json:"type"`
}

func NewOPA(policy string) (*OPA, error) {
	r := rego.New(
		rego.Query("x = data.sansshell.authz.allow"),
		rego.Module("builtin-policy.rego", policy),
	)
	pe, err := r.PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("compiling policy: %w", err)
	}
	return &OPA{policy: pe}, nil
}
func (o *OPA) evalAuth(ctx context.Context, req interface{}, method string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Internal, "no incoming gRPC metadata found")
	}
	var ServerName string
	peerInfo := &PeerInfo{}
	peer, ok := peer.FromContext(ctx)
	if ok && peer.AuthInfo != nil {
		peerInfo.Network, peerInfo.Address = peer.Addr.Network(), peer.Addr.String()
		switch peer.AuthInfo.AuthType() {
		case credentials.TLSInfo{}.AuthType():
			tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
			if !ok {
				break
			}

			if tlsInfo.SPIFFEID != nil {
				peerInfo.CertInfo.SPIFFEID = tlsInfo.SPIFFEID.String()
			}
			ServerName = tlsInfo.State.ServerName
			if len(tlsInfo.State.PeerCertificates) > 0 {
				cert := tlsInfo.State.PeerCertificates[0]
				peerInfo.CertInfo.Subject = cert.Subject
				peerInfo.CertInfo.Issuer = cert.Issuer
				peerInfo.CertInfo.DNSNames = cert.DNSNames
			}
		}
	}
	m, ok := req.(protoreflect.ProtoMessage)
	if !ok {
		return status.Error(codes.Internal, "cast to proto message failed")
	}
	msgJSON, err := protojson.Marshal(m)
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("marshaling request: %s", err))
	}
	msgRaw := json.RawMessage(msgJSON)
	log.Println("peer info = ", peerInfo)
	log.Println("peer subject serial = ", peerInfo.CertInfo.Subject.SerialNumber)

	input := input{
		Peer:       peerInfo,
		Metadata:   md,
		ServerName: ServerName,
		Method:     method,
		Message:    msgRaw,
		Type:       m.ProtoReflect().Descriptor().FullName(),
	}
	results, err := o.policy.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("error evaluating OPA policy: %s", err))
	}
	if len(results) == 0 {
		return status.Error(codes.Internal, "OPA policy result was undefined")
	}

	result, ok := results[0].Bindings["x"].(bool)
	if !ok {
		return status.Error(codes.Internal, fmt.Sprintf("OPA policy returned undefined result type: %+v", result))
	}
	if !result {
		log.Printf("Permission Denied: %+v\n", input)
		return status.Error(codes.PermissionDenied, "OPA policy does not permit this request")
	}
	return nil
}

func (o *OPA) Authorize(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := o.evalAuth(ctx, req, info.FullMethod); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

type embeddedStream struct {
	grpc.ServerStream
	info *grpc.StreamServerInfo
	o    *OPA
}

func (e *embeddedStream) RecvMsg(req interface{}) error {
	ctx := e.Context()
	// Get the actual message since at this point we just have an empty
	// one to fill in:
	if err := e.ServerStream.RecvMsg(req); err != nil {
		return err
	}
	if err := e.o.evalAuth(ctx, req, e.info.FullMethod); err != nil {
		return err
	}
	return nil
}

func (o *OPA) AuthorizeStream(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	wrapper := &embeddedStream{
		ServerStream: ss,
		info:         info,
		o:            o,
	}
	return handler(srv, wrapper)
}
