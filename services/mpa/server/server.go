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

// Package server implements the sansshell 'Mpa' service.
package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/opa/proxiedidentity"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/services"
	"github.com/Snowflake-Labs/sansshell/services/mpa"
	"github.com/Snowflake-Labs/sansshell/services/mpa/mpahooks"
	"github.com/gowebpki/jcs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	// We hardcode a maximum number of approvals to remember to avoid
	// providing an easy way to overload the server.
	maxMPAApprovals = 1000
	// We also hardcode a max age to prevent unreasonably-old approvals from being used.
	maxMPAApprovedAge = 24 * time.Hour
)

// ServerMPAAuthzHook populates approver information based on an internal MPA store.
func ServerMPAAuthzHook() rpcauth.RPCAuthzHook {
	return rpcauth.RPCAuthzHookFunc(func(ctx context.Context, input *rpcauth.RPCAuthInput) error {
		mpaID, ok := mpahooks.MPAFromIncomingContext(ctx)
		if !ok {
			// Nothing to look up if MPA wasn't requested
			return nil
		}
		resp, err := serverSingleton.Get(ctx, &mpa.GetRequest{Id: mpaID})
		if err != nil {
			return err
		}
		if resp.Action.Method != input.Method {
			// Proxies may make extra calls to the server as part of authz hooks. If
			// we get an MPA id that corresponds to a different method than the one
			// being called, it's probably from the proxy and can be ignored.
			// The right method but wrong args indicates a bigger issue and checked below.
			return nil
		}

		if err := mpahooks.ActionMatchesInput(ctx, resp.Action, input); err != nil {
			return err
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

// actionId generates the id for an action by hashing it
func actionId(action *mpa.Action) (string, error) {
	// Binary proto encoding doesn't provide any guarantees about deterministic
	// output for the same input. Go provides a deterministic marshalling option,
	// but this marshalling isn't guaranteed to be stable over time.
	// JSON encoding can be made deterministic by canonicalizing.
	b, err := protojson.Marshal(action)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(b)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(canonical)
	sum := h.Sum(nil)
	// Humans are going to need to deal with these, so let's shorten them
	// and make them a bit prettier.
	return fmt.Sprintf("%x-%x-%x", sum[0:4], sum[4:8], sum[8:12]), nil

}

type storedAction struct {
	action       *mpa.Action
	lastModified time.Time
	approvers    []*mpa.Principal
	approved     chan struct{}
}

// server is used to implement the gRPC server
type server struct {
	actions map[string]*storedAction
	mu      sync.Mutex
}

func callerIdentity(ctx context.Context) (*rpcauth.PrincipalAuthInput, bool) {
	// Prefer using a proxied identity if provided
	p := proxiedidentity.FromContext(ctx)
	if p != nil {
		return p, true
	}
	peer := rpcauth.PeerInputFromContext(ctx)
	if peer != nil {
		return peer.Principal, true
	}
	return nil, false
}

func (s *server) clearOutdatedApprovals() {
	s.mu.Lock()
	defer s.mu.Unlock()

	staleTime := time.Now().Add(-maxMPAApprovedAge)
	for id, act := range s.actions {
		if act.lastModified.Before(staleTime) {
			delete(s.actions, id)
		}
	}

}

func (s *server) Store(ctx context.Context, in *mpa.StoreRequest) (*mpa.StoreResponse, error) {
	var justification string
	if md, found := metadata.FromIncomingContext(ctx); found && len(md[rpcauth.ReqJustKey]) > 0 {
		justification = md[rpcauth.ReqJustKey][0]
	}

	p, ok := callerIdentity(ctx)
	if !ok || p == nil {
		return nil, status.Error(codes.FailedPrecondition, "unable to determine caller's identity")
	}

	action := &mpa.Action{
		User:          p.ID,
		Justification: justification,
		Method:        in.Method,
		Message:       in.Message,
	}
	id, err := actionId(action)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Time to clear out excessive approvals!
	for len(s.actions) >= maxMPAApprovals {
		var oldestID string
		oldestTime := time.Now()
		for id, act := range s.actions {
			if act.lastModified.Before(oldestTime) {
				oldestID = id
				oldestTime = act.lastModified
			}
		}
		delete(s.actions, oldestID)
	}

	act, ok := s.actions[id]
	if !ok {
		act = &storedAction{
			action:       action,
			approved:     make(chan struct{}),
			lastModified: time.Now(),
		}
		s.actions[id] = act
	}
	return &mpa.StoreResponse{
		Id:       id,
		Action:   action,
		Approver: act.approvers,
	}, nil
}

func containsPrincipal(principals []*mpa.Principal, p *rpcauth.PrincipalAuthInput) bool {
	for _, s := range principals {
		if s.Id == p.ID && reflect.DeepEqual(s.Groups, p.Groups) {
			return true
		}
	}
	return false
}

func (s *server) Approve(ctx context.Context, in *mpa.ApproveRequest) (*mpa.ApproveResponse, error) {
	p, ok := callerIdentity(ctx)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, "unable to determine caller's identity")
	}
	id, err := actionId(in.Action)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	act, ok := s.actions[id]
	if !ok {
		return nil, status.Error(codes.NotFound, "MPA request with provided input not found")
	}
	if act.action.User == p.ID {
		return nil, status.Error(codes.InvalidArgument, "MPA requests cannot be approved by their requestor")
	}
	act.lastModified = time.Now()
	// Only add the approver if it's new compared to existing approvals
	if !containsPrincipal(act.approvers, p) {
		act.approvers = append(act.approvers, &mpa.Principal{
			Id:     p.ID,
			Groups: p.Groups,
		})
		// The first approval lets any WaitForApproval calls finish immediately.
		if len(act.approvers) == 1 {
			close(act.approved)
		}
	}
	return &mpa.ApproveResponse{}, nil
}
func (s *server) WaitForApproval(ctx context.Context, in *mpa.WaitForApprovalRequest) (*mpa.WaitForApprovalResponse, error) {
	for {
		s.mu.Lock()
		act, ok := s.actions[in.Id]
		if !ok {
			return nil, status.Error(codes.NotFound, "MPA request not found")
		}
		s.mu.Unlock()
		select {
		case <-act.approved:
			return &mpa.WaitForApprovalResponse{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Minute):
			// Loop around again so that we can make sure that the request still exists
		}
	}
}
func (s *server) List(ctx context.Context, in *mpa.ListRequest) (*mpa.ListResponse, error) {
	s.clearOutdatedApprovals()
	s.mu.Lock()
	defer s.mu.Unlock()
	var items []*mpa.ListResponse_Item
	for id, action := range s.actions {
		items = append(items, &mpa.ListResponse_Item{
			Id:       id,
			Action:   action.action,
			Approver: action.approvers,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Id < items[j].Id
	})
	return &mpa.ListResponse{Item: items}, nil
}
func (s *server) Get(ctx context.Context, in *mpa.GetRequest) (*mpa.GetResponse, error) {
	s.clearOutdatedApprovals()
	s.mu.Lock()
	defer s.mu.Unlock()
	act, ok := s.actions[in.Id]
	if !ok {
		return nil, status.Error(codes.NotFound, "MPA request not found")
	}
	return &mpa.GetResponse{
		Action:   act.action,
		Approver: act.approvers,
	}, nil
}
func (s *server) Clear(ctx context.Context, in *mpa.ClearRequest) (*mpa.ClearResponse, error) {
	id, err := actionId(in.Action)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.actions, id)
	return &mpa.ClearResponse{}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	mpa.RegisterMpaServer(gs, s)
}

var serverSingleton = &server{actions: make(map[string]*storedAction)}

func init() {
	services.RegisterSansShellService(serverSingleton)
}
