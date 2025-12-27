/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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

// Package mpahooks provides grpc interceptors and other helpers for implementing MPA.
package mpahooks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/auth/proxiedidentity"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/Snowflake-Labs/sansshell/services/mpa"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	// reqMPAKey is the key name that must exist in the incoming
	// context metadata if the client wants to do an MPA request.
	reqMPAKey = "sansshell-mpa-request-id"
)

// WithMPAInMetadata adds a MPA ID to the grpc metadata of an outgoing RPC call
func WithMPAInMetadata(ctx context.Context, mpaID string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, reqMPAKey, mpaID)
}

// MPAFromIncomingContext reads a MPA ID from the grpc metadata of an incoming RPC call
func MPAFromIncomingContext(ctx context.Context) (mpaID string, ok bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	v := md.Get(reqMPAKey)
	if len(v) == 0 {
		return "", false
	}
	return v[0], true
}

// ActionMatchesInput returns an error if an MPA action doesn't match the
// message being checked in the RPCAuthInput.
func ActionMatchesInput(ctx context.Context, action *mpa.Action, input *rpcauth.RPCAuthInput) error {
	var justification string
	if j := input.Metadata[rpcauth.ReqJustKey]; len(j) > 0 {
		justification = j[0]
	}

	// Prefer using a proxied identity if provided
	var user string
	if p := proxiedidentity.FromContext(ctx); p != nil {
		user = p.ID
	} else if input.Peer != nil && input.Peer.Principal != nil {
		user = input.Peer.Principal.ID
	} else {
		return fmt.Errorf("missing peer information")
	}

	sentAct := &mpa.Action{
		User:          user,
		Method:        input.Method,
		Justification: justification,
	}

	if !action.MethodWideMpa {
		// Transform the rpcauth input into the original proto
		mt, err := protoregistry.GlobalTypes.FindMessageByURL(input.MessageType)
		if err != nil {
			return fmt.Errorf("unable to find proto type: %v", err)
		}
		m2 := mt.New().Interface()
		if err := protojson.Unmarshal([]byte(input.Message), m2); err != nil {
			return fmt.Errorf("could not marshal input into %v: %v", input.Message, err)
		}
		var msg anypb.Any
		if err := msg.MarshalFrom(m2); err != nil {
			return fmt.Errorf("unable to marshal into anyproto: %v", err)
		}

		sentAct.Message = &msg
	}
	// Make sure to use an any-proto-aware comparison
	if !cmp.Equal(action, sentAct, protocmp.Transform()) {
		return fmt.Errorf("request doesn't match mpa approval: want %v, got %v", action, sentAct)
	}
	return nil
}

func createAndBlockOnSingleTargetMPA(ctx context.Context, method string, req any, cc *grpc.ClientConn, methodWideMpa bool) (mpaID string, err error) {
	p, ok := req.(proto.Message)
	if !ok {
		return "", fmt.Errorf("unable to cast req to proto: %v", req)
	}

	var msg anypb.Any
	if err := msg.MarshalFrom(p); err != nil {
		return "", fmt.Errorf("unable to marshal into anyproto: %v", err)
	}

	mpaClient := mpa.NewMpaClient(cc)
	storeReq := &mpa.StoreRequest{
		Method:        method,
		MethodWideMpa: methodWideMpa,
	}
	if !methodWideMpa {
		storeReq.Message = &msg
	}
	result, err := mpaClient.Store(ctx, storeReq)

	if err != nil {
		return "", err
	}
	if len(result.Approver) == 0 {
		fmt.Fprintln(os.Stderr, "Multi party auth requested, ask an approver to run:")
		fmt.Fprintf(os.Stderr, "  sanssh -targets %v mpa approve %v\n", cc.Target(), result.Id)
		_, err := mpaClient.WaitForApproval(ctx, &mpa.WaitForApprovalRequest{Id: result.Id})
		if err != nil {
			return "", err
		}
	}
	return result.Id, nil
}

// UnaryClientIntercepter is a grpc.UnaryClientIntercepter that will perform the MPA flow.
func UnaryClientIntercepter(methodWideMpa bool) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Our interceptor will run for all gRPC calls, including ones used inside the interceptor.
		// We need to bail early on MPA-related ones to prevent infinite recursion.
		if method == "/Mpa.Mpa/Store" || method == "/Mpa.Mpa/WaitForApproval" {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		mpaID, err := createAndBlockOnSingleTargetMPA(ctx, method, req, cc, methodWideMpa)
		if err != nil {
			return err
		}

		ctx = WithMPAInMetadata(ctx, mpaID)
		// Complete the call
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// newStreamAfterFirstSend creates a grpc.ClientStream that doesn't attempt to begin
// the stream until SendMsg is first called. This is useful if we want to let the initial
// message affect how we set up the stream and supply metadata.
func newStreamAfterFirstSend(sendMsg func(m any) (grpc.ClientStream, error)) grpc.ClientStream {
	return &delayedStartStream{
		sendMsg:    sendMsg,
		innerReady: make(chan struct{}),
	}
}

type delayedStartStream struct {
	sendMsg    func(m any) (grpc.ClientStream, error)
	inner      grpc.ClientStream
	innerReady chan struct{}
}

func (w *delayedStartStream) SendMsg(m any) error {
	if w.inner == nil {
		s, err := w.sendMsg(m)
		if err != nil {
			return err
		}
		w.inner = s
		close(w.innerReady)
	}

	return w.inner.SendMsg(m)
}

func (w *delayedStartStream) Header() (metadata.MD, error) {
	<-w.innerReady
	return w.inner.Header()
}
func (w *delayedStartStream) Trailer() metadata.MD {
	<-w.innerReady
	return w.inner.Trailer()
}
func (w *delayedStartStream) CloseSend() error {
	<-w.innerReady
	return w.inner.CloseSend()
}
func (w *delayedStartStream) Context() context.Context {
	<-w.innerReady
	return w.inner.Context()
}
func (w *delayedStartStream) RecvMsg(m any) error {
	<-w.innerReady
	return w.inner.RecvMsg(m)
}

// StreamClientIntercepter is a grpc.StreamClientInterceptor that will perform
// the MPA flow.
func StreamClientIntercepter(methodWideMpa bool) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if method == "/Proxy.Proxy/Proxy" {
			// No need to intercept proxying, that's handled specially.
			return streamer(ctx, desc, cc, method, opts...)
		}

		return newStreamAfterFirstSend(func(m any) (grpc.ClientStream, error) {
			// Figure out the MPA request
			mpaID, err := createAndBlockOnSingleTargetMPA(ctx, method, m, cc, methodWideMpa)
			if err != nil {
				return nil, err
			}

			// Now establish the stream we actually want because we can only do so after
			// we put the MPA ID in the metadata.
			ctx := WithMPAInMetadata(ctx, mpaID)
			return streamer(ctx, desc, cc, method, opts...)
		}), nil
	}
}

func createAndBlockOnProxiedMPA(ctx context.Context, method string, args any, conn *proxy.Conn, state *util.ExecuteState, methodWideMpa bool) (mpaID string, err error) {
	p, ok := args.(proto.Message)
	if !ok {
		return "", fmt.Errorf("unable to cast args to proto: %v", args)
	}
	var msg anypb.Any
	if err := msg.MarshalFrom(p); err != nil {
		return "", fmt.Errorf("unable to marshal into anyproto: %v", err)
	}
	mpaClient := mpa.NewMpaClientProxy(conn)
	storeReq := &mpa.StoreRequest{
		Method: method,
	}
	if !methodWideMpa {
		storeReq.Message = &msg
	}
	ch, err := mpaClient.StoreOneMany(ctx, storeReq)
	if err != nil {
		return "", err
	}
	mpaIdToTargets := make(map[string][]string)
	var targetsNeedingApproval []string
	for r := range ch {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Unable to request MPA: %v\n", r.Error)
		}
		mpaIdToTargets[r.Resp.Id] = append(mpaIdToTargets[r.Resp.Id], r.Target)
		if len(r.Resp.Approver) == 0 {
			// Only print out messages for not-yet-approved requests
			targetsNeedingApproval = append(targetsNeedingApproval, r.Target)
		}
		mpaID = r.Resp.Id
	}

	if len(mpaIdToTargets) > 1 {
		var idMsgs []string
		for id, targets := range mpaIdToTargets {
			sort.Strings(targets)
			idMsgs = append(idMsgs, id+": "+strings.Join(targets, ","))
		}
		sort.Strings(idMsgs)
		for _, m := range idMsgs {
			fmt.Fprintln(os.Stderr, m)
		}
		return "", errors.New("Multiple MPA IDs generated, command needs to be run separately for each id.")
	}

	if len(targetsNeedingApproval) > 0 {
		fmt.Fprintln(os.Stderr, "Waiting for multi-party approval on all targets, ask an approver to run:")
		fmt.Fprintf(os.Stderr, "  sanssh %v-proxy %v -targets %v mpa approve %v\n", getSourceParam(state.CredSource), conn.Proxy().Target(), strings.Join(targetsNeedingApproval, ","), mpaID)
		// We call WaitForApproval on all targets, even ones already approved. This is silly but not harmful.
		waitCh, err := mpaClient.WaitForApprovalOneMany(ctx, &mpa.WaitForApprovalRequest{Id: mpaID})
		if err != nil {
			return "", err
		}
		for r := range waitCh {
			if r.Error != nil {
				fmt.Fprintf(state.Err[r.Index], "Error when waiting for MPA approval: %v\n", r.Error)
			}
		}
	}
	return mpaID, nil
}

func getSourceParam(credentialSource string) string {
	if credentialSource != "" {
		return fmt.Sprintf("--credential-source %s ", credentialSource)
	}
	return ""
}

// ProxyClientUnaryInterceptor will perform the MPA flow prior to making the desired RPC
// calls through the proxy.
func ProxyClientUnaryInterceptor(state *util.ExecuteState, methodWideMpa bool) proxy.UnaryInterceptor {
	return func(ctx context.Context, conn *proxy.Conn, method string, args any, invoker proxy.UnaryInvoker, opts ...grpc.CallOption) (<-chan *proxy.Ret, error) {
		// Our hook will run for all gRPC calls, including ones used inside the interceptor.
		// We need to bail early on MPA-related ones to prevent infinite recursion.
		if method == "/Mpa.Mpa/Store" || method == "/Mpa.Mpa/WaitForApproval" {
			return invoker(ctx, method, args, opts...)
		}

		mpaID, err := createAndBlockOnProxiedMPA(ctx, method, args, conn, state, methodWideMpa)
		if err != nil {
			return nil, err
		}

		// Now that we have our approvals, make our call.
		ctx = WithMPAInMetadata(ctx, mpaID)
		return invoker(ctx, method, args, opts...)
	}
}

// ProxyClientStreamInterceptor will perform the MPA flow prior to making the desired streaming
// RPC calls through the proxy.
func ProxyClientStreamInterceptor(state *util.ExecuteState, methodWideMpa bool) proxy.StreamInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *proxy.Conn, method string, streamer proxy.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return newStreamAfterFirstSend(func(args any) (grpc.ClientStream, error) {
			// Figure out the MPA request
			mpaID, err := createAndBlockOnProxiedMPA(ctx, method, args, cc, state, methodWideMpa)
			if err != nil {
				return nil, err
			}

			// Now establish the stream we actually want because we can only do so after
			// we put the MPA ID in the metadata.
			ctx := WithMPAInMetadata(ctx, mpaID)
			return streamer(ctx, desc, method, opts...)
		}), nil
	}
}

// ProxyMPAAuthzHook populates MPA information in the input message
func ProxyMPAAuthzHook() rpcauth.RPCAuthzHook {
	return rpcauth.RPCAuthzHookFunc(func(ctx context.Context, input *rpcauth.RPCAuthInput) error {
		mpaID, ok := MPAFromIncomingContext(ctx)
		if !ok {
			// No need to call out if MPA wasn't requested
			return nil
		}

		if input.Environment == nil || !input.Environment.NonHostPolicyCheck {
			// Proxies will evaluate Authz polices on both proxy-level calls and host-level
			// calls. We want to only gather MPA info for host-level calls.
			return nil
		}

		if input.TargetConn == nil {
			// If there's no host, we can't call out to the host.
			return nil
		}

		client := mpa.NewMpaClient(input.TargetConn)
		resp, err := client.Get(ctx, &mpa.GetRequest{Id: mpaID})
		if err != nil {
			return fmt.Errorf("failed getting MPA info: %v", err)
		}

		if err := ActionMatchesInput(ctx, resp.Action, input); err != nil {
			return err
		}

		if resp.Action.MethodWideMpa {
			input.ApprovedMethodWideMpa = true
		}

		for _, a := range resp.Approver {
			input.Approvers = append(input.Approvers, &rpcauth.PrincipalAuthInput{
				ID:     a.Id,
				Groups: a.Groups,
			})
		}
		return nil
	})
}
