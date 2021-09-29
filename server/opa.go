package server

// Based on this excellent Apache-licensed example by Stephan Renatus:
// https://github.com/open-policy-agent/opa/issues/1931

import (
	"context"
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

type input struct {
	Peer       *peer.Peer            `json:"peer"`
	Method     string                `json:"method"`
	ServerName string                `json:"servername"`
	SPIFFEID   string                `json:"spiffeid"`
	Metadata   metadata.MD           `json:"metadata"`
	Message    json.RawMessage       `json:"message"`
	Type       protoreflect.FullName `json:"type"`
}

func NewOPA(policy string) (*OPA, error) {
	r := rego.New(
		rego.Query("x = data.unshelled.authz.allow"),
		rego.Module("builtin-policy.rego", policy),
	)
	pe, err := r.PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("compiling policy: %w", err)
	}
	return &OPA{policy: pe}, nil
}

func (o *OPA) Authorize(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "no incoming gRPC metadata found")
	}
	var ServerName, SPIFFEID string
	peer, ok := peer.FromContext(ctx)
	if ok && peer.AuthInfo != nil {
		switch peer.AuthInfo.AuthType() {
		case credentials.TLSInfo{}.AuthType():
			tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
			if !ok {
				break
			}
			if tlsInfo.SPIFFEID != nil {
				SPIFFEID = tlsInfo.SPIFFEID.String()
			}
			ServerName = tlsInfo.State.ServerName
		}
	}
	m, ok := req.(protoreflect.ProtoMessage)
	if !ok {
		return nil, status.Error(codes.Internal, "cast to proto message failed")
	}
	msgJSON, err := protojson.Marshal(m)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("marshaling request: %s", err))
	}
	msgRaw := json.RawMessage(msgJSON)

	input := input{
		Metadata:   md,
		Peer:       peer,
		ServerName: ServerName,
		SPIFFEID:   SPIFFEID,
		Method:     info.FullMethod,
		Message:    msgRaw,
		Type:       m.ProtoReflect().Descriptor().FullName(),
	}

	results, err := o.policy.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("error evaluating OPA policy: %s", err))
	}
	if len(results) == 0 {
		return nil, status.Error(codes.Internal, "OPA policy result was undefined")
	}

	result, ok := results[0].Bindings["x"].(bool)
	if !ok {
		return nil, status.Error(codes.Internal, fmt.Sprintf("OPA policy returned undefined result type: %+v", result))
	}
	if !result {
		log.Printf("Permission Denied: %+v\n", input)
		return nil, status.Error(codes.PermissionDenied, "OPA policy does not permit this request")
	}
	return handler(ctx, req)
}
