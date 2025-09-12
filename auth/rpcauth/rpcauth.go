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

// Package rpcauth provides authz policy authorization
// for Sansshell RPCs.
package rpcauth

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
)

// Metrics
var (
	authzDeniedPolicyCounter = metrics.MetricDefinition{Name: "authz_denied_policy",
		Description: "number of authorization denied by policy"}
	authzDenialHintErrorCounter = metrics.MetricDefinition{Name: "authz_denial_hint_error",
		Description: "number of failure to get denial hint"}
	authzFailureInputMissingCounter = metrics.MetricDefinition{Name: "authz_failure_input_missing",
		Description: "number of authorization failure due to missing input"}
	authzFailureEvalErrorCounter = metrics.MetricDefinition{Name: "authz_failure_eval_error",
		Description: "number of authorization failure due to policy evaluation error"}
)

type AuthzPolicy interface {
	Eval(ctx context.Context, input *RPCAuthInput) (bool, error)
	DenialHints(ctx context.Context, input *RPCAuthInput) ([]string, error)
}

type RPCAuthorizer interface {
	// Eval will evalulate the supplied input against the authorization policy, returning
	// nil if policy evaulation was successful, and the request is permitted, or
	// an appropriate status.Error otherwise. Any input hooks will be executed
	// prior to policy evaluation, and may mutate `input`, regardless of the
	// the success or failure of policy.
	Eval(ctx context.Context, input *RPCAuthInput) error

	// Authorize implements grpc.UnaryServerInterceptor, and will authorize each rpc to a service
	Authorize(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)

	// AuthorizeStream implements grpc.StreamServerInterceptor and applies policy checks on any RecvMsg calls to a service
	AuthorizeStream(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error

	// AuthorizeClient implements grpc.UnaryClientInterceptor, and will authorize each rpc on each call to remote service
	AuthorizeClient(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error

	// AuthorizeClientStream implements grpc.StreamClientInterceptor and applies policy checks on any SendMsg calls to remote service
	AuthorizeClientStream(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error)
}

// An Authorizer performs authorization of Sanshsell RPCs based on
// an OPA/Rego policy.
//
// It can be used as both a unary and stream interceptor, or manually
// invoked to perform policy checks using `Eval`
type rpcAuthorizerImpl struct {
	// The AuthzPolicy used to perform authorization checks.
	policy AuthzPolicy

	// Additional authorization hooks invoked before policy evaluation.
	hooks []RPCAuthzHook
}

// A RPCAuthzHook is invoked on populated RpcAuthInput prior to policy
// evaluation, and may augment / mutate the input, or pre-emptively
// reject a request.
type RPCAuthzHook interface {
	Hook(context.Context, *RPCAuthInput) error
}

// NewRPCAuthorizer creates a new Authorizer with AuthzPolicy. Any supplied authorization
// hooks will be executed, in the order provided, on each policy evauluation.
// NOTE: The policy is used for both client and server hooks below. If you need
//
//	distinct policy for client vs server, create 2 Authorizer's.
func NewRPCAuthorizer(policy AuthzPolicy, authzHooks ...RPCAuthzHook) RPCAuthorizer {
	return &rpcAuthorizerImpl{
		policy: policy,
		hooks:  authzHooks,
	}
}

// Eval will evalulate the supplied input against the authorization policy, returning
// nil iff policy evaulation was successful, and the request is permitted, or
// an appropriate status.Error otherwise. Any input hooks will be executed
// prior to policy evaluation, and may mutate `input`, regardless of the
// the success or failure of policy.
func (g *rpcAuthorizerImpl) Eval(ctx context.Context, input *RPCAuthInput) error {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	redactedInput, err := getRedactedInput(input)
	if err != nil {
		return fmt.Errorf("failed to get redacted input: %v", err)
	}
	if input != nil {
		logger.V(2).Info("evaluating authz policy", "input", redactedInput)
	}
	if input == nil {
		err := status.Error(codes.InvalidArgument, "policy input cannot be nil")
		logger.V(1).Error(err, "failed to evaluate authz policy", "input", redactedInput)
		recorder.CounterOrLog(ctx, authzFailureInputMissingCounter, 1)
		return err
	}
	for _, hook := range g.hooks {
		if err := hook.Hook(ctx, input); err != nil {
			logger.V(1).Error(err, "authz hook error", "input", redactedInput)
			if _, ok := status.FromError(err); ok {
				// error is already an appropriate status.Status
				return err
			}
			return status.Errorf(codes.Internal, "authz hook error: %v", err)
		}
	}
	redactedInput, err = getRedactedInput(input)
	if err != nil {
		return fmt.Errorf("failed to get redacted input post hooks: %v", err)
	}
	logger.V(2).Info("evaluating authz policy post hooks", "input", redactedInput)
	result, err := g.policy.Eval(ctx, input)
	if err != nil {
		logger.V(1).Error(err, "failed to evaluate authz policy", "input", redactedInput)
		recorder.CounterOrLog(ctx, authzFailureEvalErrorCounter, 1, attribute.String("method", input.Method))
		return status.Errorf(codes.Internal, "authz policy evaluation error: %v", err)
	}
	var hints []string
	if !result {
		// We've failed so let's see if we can help tell the user what might have failed.
		hints, err = g.policy.DenialHints(ctx, input)
		if err != nil {

			recorder.CounterOrLog(ctx, authzDenialHintErrorCounter, 1, attribute.String("method", input.Method))
			// We can't do much here besides log that something went wrong
			logger.V(1).Error(err, "failed to get hints for authz policy denial", "error", err)
		}
	}
	logger.Info("authz policy evaluation result", "authorizationResult", result, "input", redactedInput, "denialHints", hints)
	if !result {
		errRegister := recorder.Counter(ctx, authzDeniedPolicyCounter, 1, attribute.String("method", input.Method))
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to add counter "+authzDeniedPolicyCounter.Name)
		}
		if len(hints) > 0 {
			if len(hints) == 1 {
				return status.Errorf(codes.PermissionDenied, "Authz policy does not permit this request: %v", hints[0])
			}

			errorMessageParts := []string{
				"Authz policy does not permit this request, there are several reasons why:\n",
			}
			for _, hint := range hints {
				errorMessageParts = append(errorMessageParts, "  * ", hint, "\n")
			}
			return status.Errorf(codes.PermissionDenied, strings.Join(errorMessageParts, ""))
		} else {
			return status.Errorf(codes.PermissionDenied, "Authz policy does not permit this request")
		}
	}
	return nil
}

// Authorize implements grpc.UnaryServerInterceptor
func (g *rpcAuthorizerImpl) Authorize(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	msg, ok := req.(proto.Message)
	if !ok {
		return nil, status.Errorf(codes.Internal, "unable to authorize request of type %T which is not proto.Message", req)
	}
	authInput, err := NewRPCAuthInput(ctx, info.FullMethod, msg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to create auth input: %v", err)
	}
	if err := g.Eval(ctx, authInput); err != nil {
		return nil, err
	}
	ctx = AddPeerToContext(ctx, authInput.Peer)
	return handler(ctx, req)
}

// AuthorizeClient implements grpc.UnaryClientInterceptor
func (g *rpcAuthorizerImpl) AuthorizeClient(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	msg, ok := req.(proto.Message)
	if !ok {
		return status.Errorf(codes.Internal, "unable to authorize request of type %T which is not proto.Message", req)
	}
	authInput, err := NewRPCAuthInput(ctx, method, msg)
	if err != nil {
		return status.Errorf(codes.Internal, "unable to create auth input: %v", err)
	}
	if err := g.Eval(ctx, authInput); err != nil {
		return err
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

// AuthorizeClientStream implements grpc.StreamClientInterceptor and applies policy checks on any SendMsg calls.
func (g *rpcAuthorizerImpl) AuthorizeClientStream(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	clientStream, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't create clientStream: %v", err)
	}
	wrapped := &wrappedClientStream{
		ClientStream: clientStream,
		method:       method,
		authz:        g,
	}
	return wrapped, nil
}

// wrappedClientStream wraps an existing grpc.ClientStream with authorization checking.
type wrappedClientStream struct {
	grpc.ClientStream
	method string
	authz  *rpcAuthorizerImpl
}

// see: grpc.ClientStream.SendMsg
func (e *wrappedClientStream) SendMsg(req interface{}) error {
	ctx := e.Context()
	msg, ok := req.(proto.Message)
	if !ok {
		return status.Errorf(codes.Internal, "unable to authorize request of type %T which is not proto.Message", req)
	}
	authInput, err := NewRPCAuthInput(ctx, e.method, msg)
	if err != nil {
		return err
	}
	if err := e.authz.Eval(ctx, authInput); err != nil {
		return err
	}
	return e.ClientStream.SendMsg(req)
}

// AuthorizeStream implements grpc.StreamServerInterceptor and applies policy checks on any RecvMsg calls.
func (g *rpcAuthorizerImpl) AuthorizeStream(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	wrapped := &wrappedStream{
		ServerStream: ss,
		info:         info,
		authz:        g,
	}
	return handler(srv, wrapped)
}

// wrappedStream wraps an existing grpc.ServerStream with authorization checking.
type wrappedStream struct {
	grpc.ServerStream
	info  *grpc.StreamServerInfo
	authz *rpcAuthorizerImpl

	peerMu            sync.Mutex
	lastPeerAuthInput *PeerAuthInput
}

func (e *wrappedStream) Context() context.Context {
	e.peerMu.Lock()
	ctx := AddPeerToContext(e.ServerStream.Context(), e.lastPeerAuthInput)
	e.peerMu.Unlock()
	return ctx
}

// see: grpc.ServerStream.RecvMsg
func (e *wrappedStream) RecvMsg(req interface{}) error {
	ctx := e.Context()
	// The argument to RecvMsg is the destination message, which will
	// be filled by the stream.
	// Therefore, in order to check the message against the policy, it
	// first needs to be populated by receiving from the wire.
	if err := e.ServerStream.RecvMsg(req); err != nil {
		return err
	}
	msg, ok := req.(proto.Message)
	if !ok {
		return status.Errorf(codes.Internal, "unable to authorize request of type %T which is not proto.Message", req)
	}
	authInput, err := NewRPCAuthInput(ctx, e.info.FullMethod, msg)
	if err != nil {
		return err
	}
	if err := e.authz.Eval(ctx, authInput); err != nil {
		return err
	}
	e.peerMu.Lock()
	e.lastPeerAuthInput = authInput.Peer
	e.peerMu.Unlock()
	return nil
}
