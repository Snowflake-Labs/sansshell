// package rpcauth provides OPA policy authorization
// for Sansshell RPCs.
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

	// Additional authorization hooks invoked before policy evaluation.
	hooks []RpcAuthzHook
}

// An RpcAuthzHook is invoked on populated RpcAuthInput prior to policy
// evaluation, and may augment / mutate the input, or pre-emptively
// reject a request.
type RpcAuthzHook interface {
	Apply(context.Context, *RpcAuthInput) error
}

// AuthzHookFunc implements RpcAuthzHook for a simple function
type RpcAuthzHookFunc func(context.Context, *RpcAuthInput) error

func (r RpcAuthzHookFunc) Apply(ctx context.Context, input *RpcAuthInput) error {
	return r(ctx, input)
}

// New creates a new Authorizer from an opa.AuthzPolicy
func New(policy *opa.AuthzPolicy, inputHooks ...RpcAuthzHook) *Authorizer {
	return &Authorizer{policy: policy, hooks: inputHooks}
}

// NewWithPolicy creates a new Authorizer from a policy string.
func NewWithPolicy(ctx context.Context, policy string, inputHooks ...RpcAuthzHook) (*Authorizer, error) {
	p, err := opa.NewAuthzPolicy(ctx, policy)
	if err != nil {
		return nil, err
	}
	return New(p, inputHooks...), nil
}

// Evalulate the supplied input against the authorization policy, returning
// nil iff policy evaulation was successful, and the request is permitted, or
// an appropriate status.Error otherwise. Any input hooks will be executed
// prior to policy evaluation, and may mutate `input`, regardless of the
// the success or failure of policy.
func (g *Authorizer) Eval(ctx context.Context, input *RpcAuthInput) error {
	if input == nil {
		return status.Error(codes.InvalidArgument, "policy input cannot be nil")
	}
	for _, hook := range g.hooks {
		if err := hook.Apply(ctx, input); err != nil {
			if _, ok := status.FromError(err); ok {
				// error is already an appropriate status.Status
				return err
			}
			return status.Errorf(codes.Internal, "authz hook error: %v", err)
		}
	}
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

// wrappedStream wraps an existing grpc.ServerStream with authorization checking.
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
