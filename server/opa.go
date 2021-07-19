package server

// Based on this excellent Apache-licensed example by Stephan Renatus:
// https://github.com/open-policy-agent/opa/issues/1931

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/open-policy-agent/opa/rego"
)

type OPA struct {
	policy rego.PreparedEvalQuery
}

type input struct {
	Metadata metadata.MD           `json:"metadata"`
	Message  interface{}           `json:"message"`
	Type     protoreflect.FullName `json:"type"`
}

func NewOPA(policy string) (*OPA, error) {
	r := rego.New(
		rego.Query("x = data.unshelled.authz.allow"),
		rego.Module("builtin-policy.rego", policy),
	)
	pe, err := r.PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("compiling policy: %s", err)
	}
	return &OPA{policy: pe}, nil
}

func (o *OPA) Authorize(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	m, ok := req.(protoreflect.ProtoMessage)
	if !ok {
		return nil, fmt.Errorf("cast to proto message failed")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no incoming gRPC metadata found")
	}

	input := input{
		Metadata: md,
		Message:  req,
		Type:     m.ProtoReflect().Descriptor().FullName(),
	}
	fmt.Printf("Evaluating input: %+v\n", input)

	results, err := o.policy.Eval(ctx, rego.EvalInput(input))

	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("error evaluating OPA policy: %s", err))
	} else if len(results) == 0 {
		return nil, status.Error(codes.Internal, "OPA policy result was undefined")
	} else if result, ok := results[0].Bindings["x"].(bool); !ok {
		return nil, status.Error(codes.Internal, fmt.Sprintf("OPA policy returned undefined result type: %+v", result))
	} else {
		if result {
			return handler(ctx, req)
		} else {
			return nil, status.Error(codes.PermissionDenied, "OPA policy does not permit this request")
		}
	}
	return nil, status.Error(codes.Internal, "logic error in rego error handling")
}
