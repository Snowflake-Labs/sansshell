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

package server

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	"github.com/Snowflake-Labs/sansshell/services/mpa"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func mustAny(a *anypb.Any, err error) *anypb.Any {
	if err != nil {
		panic(err)
	}
	return a
}

func TestAuthzHook(t *testing.T) {
	ctx := context.Background()

	// Create a hook and make sure that hooking with no mpa request works
	hook := ServerMPAAuthzHook()
	newInput, err := rpcauth.NewRPCAuthInput(ctx, "foobar", &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if err := hook.Hook(ctx, newInput); err != nil {
		t.Fatal(err)
	}

	// Add a request
	rCtx := rpcauth.AddPeerToContext(ctx, &rpcauth.PeerAuthInput{
		Principal: &rpcauth.PrincipalAuthInput{ID: "requester"},
	})
	if _, err := serverSingleton.Store(rCtx, &mpa.StoreRequest{
		Method:  "foobar",
		Message: mustAny(anypb.New(&emptypb.Empty{})),
	}); err != nil {
		t.Fatal(err)
	}

	// Make sure we can't approve our own request
	approvReq := &mpa.ApproveRequest{
		Action: &mpa.Action{
			User:    "requester",
			Method:  "foobar",
			Message: mustAny(anypb.New(&emptypb.Empty{})),
		},
	}
	if _, err := serverSingleton.Approve(rCtx, approvReq); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected failure when self-approving: %v", err)
	}

	aCtx := rpcauth.AddPeerToContext(ctx, &rpcauth.PeerAuthInput{
		Principal: &rpcauth.PrincipalAuthInput{ID: "approver", Groups: []string{"g1"}},
	})
	// Approve the request twice to make sure approval is idempotent
	for i := 0; i < 2; i++ {
		if _, err := serverSingleton.Approve(aCtx, approvReq); err != nil {
			t.Fatal(err)
		}
	}

	mpaCtx := metadata.NewIncomingContext(rCtx, map[string][]string{"sansshell-mpa-request-id": {"3e31b2b4-f8724bae-c1504987-0c3245bf-a6e2ed58-521535e1-81dfe77b-2e95844c"}})
	passingInput, err := rpcauth.NewRPCAuthInput(mpaCtx, "foobar", &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if err := hook.Hook(mpaCtx, passingInput); err != nil {
		t.Fatal(err)
	}
	wantApprovers := []*rpcauth.PrincipalAuthInput{
		{ID: "approver", Groups: []string{"g1"}},
	}
	if !reflect.DeepEqual(passingInput.Approvers, wantApprovers) {
		t.Errorf("got %+v, want %+v", passingInput.Approvers, wantApprovers)
	}

	// An action not matching the input should fail
	wrongInput, err := rpcauth.NewRPCAuthInput(mpaCtx, "foobar", &mpa.ApproveRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := hook.Hook(mpaCtx, wrongInput); err == nil {
		t.Fatal("unexpectedly nil err")
	}
}

func TestApproveDedupIsGroupOrderInsensitive(t *testing.T) {
	ctx := context.Background()

	rCtx := rpcauth.AddPeerToContext(ctx, &rpcauth.PeerAuthInput{
		Principal: &rpcauth.PrincipalAuthInput{ID: "requester"},
	})
	storeResp, err := serverSingleton.Store(rCtx, &mpa.StoreRequest{
		Method:  "group-order",
		Message: mustAny(anypb.New(&emptypb.Empty{})),
	})
	if err != nil {
		t.Fatal(err)
	}

	approveReq := &mpa.ApproveRequest{
		Action: &mpa.Action{
			User:    "requester",
			Method:  "group-order",
			Message: mustAny(anypb.New(&emptypb.Empty{})),
		},
	}

	// The same approver approves twice, presenting the same group set in a
	// different order. Both must collapse to a single approver entry.
	for _, groups := range [][]string{{"g1", "g2"}, {"g2", "g1"}} {
		aCtx := rpcauth.AddPeerToContext(ctx, &rpcauth.PeerAuthInput{
			Principal: &rpcauth.PrincipalAuthInput{ID: "approver", Groups: groups},
		})
		if _, err := serverSingleton.Approve(aCtx, approveReq); err != nil {
			t.Fatal(err)
		}
	}

	getResp, err := serverSingleton.Get(ctx, &mpa.GetRequest{Id: storeResp.Id})
	if err != nil {
		t.Fatal(err)
	}
	if len(getResp.Approver) != 1 {
		t.Fatalf("got %d approvers, want 1 (group order must not create duplicates): %+v", len(getResp.Approver), getResp.Approver)
	}
}

func TestActionIdIsFullDigest(t *testing.T) {
	id, err := actionId(&mpa.Action{User: "u", Method: "m"})
	if err != nil {
		t.Fatal(err)
	}
	// A full SHA-256 digest is 32 bytes -> 64 hex chars. The id must encode the
	// entire digest (not a truncated prefix), so that collisions stay infeasible.
	hexOnly := strings.ReplaceAll(id, "-", "")
	if len(hexOnly) != 64 {
		t.Fatalf("action id encodes %d hex chars, want 64 (full 256-bit digest): %q", len(hexOnly), id)
	}
}

func TestMaxNumApprovals(t *testing.T) {
	ctx := context.Background()
	for i := 0; i < maxMPAApprovals+20; i++ {
		rCtx := rpcauth.AddPeerToContext(ctx, &rpcauth.PeerAuthInput{
			Principal: &rpcauth.PrincipalAuthInput{ID: "requester"},
		})
		if _, err := serverSingleton.Store(rCtx, &mpa.StoreRequest{
			Method:  "foobar" + strconv.Itoa(i),
			Message: mustAny(anypb.New(&emptypb.Empty{})),
		}); err != nil {
			t.Fatal(err)
		}
	}
	reqs, err := serverSingleton.List(ctx, &mpa.ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs.Item) != maxMPAApprovals {
		t.Fatalf("got %v requests, expected %v", len(reqs.Item), maxMPAApprovals)
	}
}

func TestWaitForApproval(t *testing.T) {
	ctx := context.Background()

	rCtx := rpcauth.AddPeerToContext(ctx, &rpcauth.PeerAuthInput{
		Principal: &rpcauth.PrincipalAuthInput{ID: "requester"},
	})
	if _, err := serverSingleton.Store(rCtx, &mpa.StoreRequest{
		Method:  "foobar",
		Message: mustAny(anypb.New(&emptypb.Empty{})),
	}); err != nil {
		t.Fatal(err)
	}

	var g errgroup.Group
	g.Go(func() error {
		aCtx := rpcauth.AddPeerToContext(ctx, &rpcauth.PeerAuthInput{
			Principal: &rpcauth.PrincipalAuthInput{ID: "approver", Groups: []string{"g1"}},
		})
		_, err := serverSingleton.Approve(aCtx, &mpa.ApproveRequest{
			Action: &mpa.Action{
				User:    "requester",
				Method:  "foobar",
				Message: mustAny(anypb.New(&emptypb.Empty{})),
			},
		})
		return err
	})

	_, err := serverSingleton.WaitForApproval(ctx, &mpa.WaitForApprovalRequest{
		Id: "3e31b2b4-f8724bae-c1504987-0c3245bf-a6e2ed58-521535e1-81dfe77b-2e95844c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForApprovalNotFound(t *testing.T) {
	ctx := context.Background()

	_, err := serverSingleton.WaitForApproval(ctx, &mpa.WaitForApprovalRequest{
		Id: "3e31b2b4-f8724bae-not-found",
	})
	if err == nil {
		t.Fatal("expected non nil err")
	} else if status.Convert(err).Code() != codes.NotFound {
		t.Fatal("expected not found code")
	}

	// Call store only later to check if we don't have a case of a "deadlock" (or close).
	rCtx := rpcauth.AddPeerToContext(ctx, &rpcauth.PeerAuthInput{
		Principal: &rpcauth.PrincipalAuthInput{ID: "requester"},
	})
	if _, err := serverSingleton.Store(rCtx, &mpa.StoreRequest{
		Method:  "foobar",
		Message: mustAny(anypb.New(&emptypb.Empty{})),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestActionIdIsDeterministic(t *testing.T) {
	for _, tc := range []struct {
		desc   string
		action *mpa.Action
		wantID string
	}{
		{
			desc:   "empty action",
			action: &mpa.Action{},
			wantID: "44136fa3-55b3678a-1146ad16-f7e8649e-94fb4fc2-1fe77e83-10c060f6-1caaff8a",
		},
		{
			desc: "simple action",
			action: &mpa.Action{
				User:    "requester",
				Method:  "foobar",
				Message: mustAny(anypb.New(&emptypb.Empty{})),
			},
			wantID: "3e31b2b4-f8724bae-c1504987-0c3245bf-a6e2ed58-521535e1-81dfe77b-2e95844c",
		},
		{
			desc: "complex action",
			action: &mpa.Action{
				User:          "user",
				Method:        "method",
				Justification: "justification",
				Message: mustAny(anypb.New(&mpa.Action{
					User:          "so",
					Method:        "meta",
					Justification: "nested",
					Message: mustAny(anypb.New(&mpa.Principal{
						Id:     "approver",
						Groups: []string{"g1", "g2"},
					})),
				})),
			},
			wantID: "66bc8827-d4fab1bf-b51181f1-99022e7f-8764af91-25f06cf0-f2c5beca-39ff3098",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			id, err := actionId(tc.action)
			if err != nil {
				t.Error(err)
			}
			if id != tc.wantID {
				t.Errorf("got %v, want %v", id, tc.wantID)
			}
		})
	}
}
