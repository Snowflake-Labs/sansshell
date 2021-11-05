// package rpcauth provides OPA policy authorization
// for RPC
package rpcauth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/Snowflake-Labs/sansshell/auth/opa"
)

// An Authorizer performs authorization of Sanshsell RPCs
//
// It can be used as both a unary and stream interceptor, or manually
// invoked to perform policy checks using `Eval`
type Authorizer struct {
	// The AuthzPolicy used to perform authorization checks.
	policy *opa.AuthzPolicy
}

// An RpcAuthInputHook is a function that mutates or
// augments RpcAuthInput data
type RpcAuthInputHook func(*RpcAuthInput) error

// New creates a new Authorizer from an opa.AuthzPolicy
func New(policy *opa.AuthzPolicy) *Authorizer {
	return &Authorizer{policy: policy}
}

// NewWithPolicy creates a new Authorizer from a policy string.
func NewWithPolicy(ctx context.Context, policy string) (*Authorizer, error) {
	p, err := opa.NewAuthzPolicy(ctx, policy)
	if err != nil {
		return nil, err
	}
	return New(p), nil
}

// Evalulate the supplied input against the authorization policy, returning
// nil iff policy evaulation was successful, and the request is permitted, or
// an appropriate status.Error otherwise.
func (g *Authorizer) Eval(ctx context.Context, input *RpcAuthInput) error {
	allowed, err := g.policy.Eval(ctx, input)
	if err != nil {
		return status.Errorf(codes.Internal, "authz policy evaluation error: %v", err)
	}
	if !allowed {
		return status.Errorf(codes.PermissionDenied, "OPA policy does not permit this request")
	}
	return nil
}

// Authorize implements grpc.UnaryServerInterceptor
func (g *Authorizer) Authorize(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	msg, ok := req.(proto.Message)
	if !ok {
		return nil, status.Errorf(codes.Internal, "unable to authorize request of type %T, which is not proto.Message", req)
	}
	authInput, err := NewRpcAuthInput(ctx, info.FullMethod, msg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to create auth input: %v", err)
	}
	if err := g.Eval(ctx, authInput); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// AuthorizeStream implements grpc.StreamServerInterceptor
func (c *Authorizer) AuthorizeStream(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	wrapped := &wrappedStream{
		ServerStream: ss,
		info:         info,
		authz:        c,
	}
	return handler(srv, wrapped)
}

type wrappedStream struct {
	grpc.ServerStream
	info  *grpc.StreamServerInfo
	authz *Authorizer
}

// see: grpc.ServerStream.RecvMsg
func (e *wrappedStream) RecvMsg(req interface{}) error {
	ctx := e.Context()
	if err := e.ServerStream.RecvMsg(req); err != nil {
		return err
	}
	msg, ok := req.(proto.Message)
	if !ok {
		return status.Errorf(codes.Internal, "unable to authorize request of type %T, which is not proto.Message", req)
	}
	authInput, err := NewRpcAuthInput(ctx, e.info.FullMethod, msg)
	if err != nil {
		return err
	}
	if err := e.authz.Eval(ctx, authInput); err != nil {
		return err
	}
	return nil
}
