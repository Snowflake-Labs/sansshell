package rpcauth

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/auth/opa"
)

func TestAuthzHook(t *testing.T) {
	ctx := context.Background()
	policyString := `
package sansshell.authz

default allow = false

allow {
  input.method = "/Foo.Bar/Baz"
  input.type = "Foo.BazRequest"
}

allow {
  input.peer.principal.id = "admin@foo"
}

allow {
  some i
  input.peer.principal.groups[i] = "admin_users"
}

`
	policy, err := opa.NewAuthzPolicy(ctx, policyString)
	if err != nil {
		t.Fatalf("NewAuthzPolicy(%v), err was %v, want nil", policyString, err)
	}

	wantStatusCode := func(c codes.Code) func(*testing.T, error) {
		return func(t *testing.T, err error) {
			t.Helper()
			if status.Code(err) != c {
				t.Errorf("err was %v, want err with code %s", err, c)
			}
		}
	}

	for _, tc := range []struct {
		name    string
		input   *RpcAuthInput
		hooks   []RpcAuthzHook
		errFunc func(*testing.T, error)
	}{
		{
			name:    "nil input, no hooks",
			input:   nil,
			hooks:   []RpcAuthzHook{},
			errFunc: wantStatusCode(codes.InvalidArgument),
		},
		{
			name:    "empty input, no hooks, deny",
			input:   &RpcAuthInput{},
			hooks:   []RpcAuthzHook{},
			errFunc: wantStatusCode(codes.PermissionDenied),
		},
		{
			name:  "single hook, create allow",
			input: &RpcAuthInput{},
			hooks: []RpcAuthzHook{
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					input.MessageType = "Foo.BazRequest"
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name:  "multiple hooks, create allow",
			input: &RpcAuthInput{},
			hooks: []RpcAuthzHook{
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					return nil
				}),
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					input.MessageType = "Foo.BazRequest"
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name:  "single hook, hook reject with code",
			input: &RpcAuthInput{},
			hooks: []RpcAuthzHook{
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					return status.Error(codes.FailedPrecondition, "hook failed")
				}),
			},
			errFunc: wantStatusCode(codes.FailedPrecondition),
		},
		{
			name:  "multi-hook, first hook rejects with code",
			input: &RpcAuthInput{},
			hooks: []RpcAuthzHook{
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					return status.Error(codes.FailedPrecondition, "hook failed")
				}),
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					// never invoked
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.FailedPrecondition),
		},
		{
			name:  "multi-hook, last hook rejects with code",
			input: &RpcAuthInput{},
			hooks: []RpcAuthzHook{
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					return nil
				}),
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					return status.Error(codes.FailedPrecondition, "hook failed")
				}),
			},
			errFunc: wantStatusCode(codes.FailedPrecondition),
		},
		{
			name:  "single hook, hook reject without code",
			input: &RpcAuthInput{},
			hooks: []RpcAuthzHook{
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					return errors.New("non status")
				}),
			},
			errFunc: wantStatusCode(codes.Internal),
		},
		{
			name:  "hook ordering",
			input: &RpcAuthInput{},
			hooks: []RpcAuthzHook{
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					input.MessageType = "Foo.BarRequest"
					return nil
				}),
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					input.MessageType = "Foo.BazRequest"
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name:  "synthesize data, allow",
			input: &RpcAuthInput{Method: "/Foo.Bar/Foo"},
			hooks: []RpcAuthzHook{
				RpcAuthzHookFunc(func(ctx context.Context, input *RpcAuthInput) error {
					if input.Peer == nil {
						input.Peer = &PeerAuthInput{
							Principal: &PrincipalAuthInput{
								ID: "admin@foo",
							},
						}
					}
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			authz := New(policy, tc.hooks...)
			err := authz.Eval(ctx, tc.input)
			tc.errFunc(t, err)
		})
	}
}
