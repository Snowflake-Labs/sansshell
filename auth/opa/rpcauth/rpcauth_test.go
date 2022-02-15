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

package rpcauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

var policyString = `
package sansshell.authz

default allow = false

allow {
  input.method = "/Foo.Bar/Baz"
  input.type = "Foo.BazRequest"
}

allow {
  input.method = "/Foo/Bar"
}

allow {
  input.peer.principal.id = "admin@foo"
}

allow {
  input.host.net.address = "127.0.0.1"
  input.host.net.port = "1"
}

allow {
  some i
  input.peer.principal.groups[i] = "admin_users"
}

`

func TestAuthzHook(t *testing.T) {
	ctx := context.Background()
	policy, err := opa.NewAuthzPolicy(ctx, policyString)
	testutil.FatalOnErr("NewAuthzPolicy", err, t)

	tcp, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:1")
	testutil.FatalOnErr("ResolveIPAddr", err, t)

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
		input   *RPCAuthInput
		hooks   []RPCAuthzHook
		errFunc func(*testing.T, error)
	}{
		{
			name:    "nil input, no hooks",
			input:   nil,
			hooks:   []RPCAuthzHook{},
			errFunc: wantStatusCode(codes.InvalidArgument),
		},
		{
			name:    "empty input, no hooks, deny",
			input:   &RPCAuthInput{},
			hooks:   []RPCAuthzHook{},
			errFunc: wantStatusCode(codes.PermissionDenied),
		},
		{
			name:  "single hook, create allow",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					input.MessageType = "Foo.BazRequest"
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name:  "multiple hooks, create allow",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					return nil
				}),
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					input.MessageType = "Foo.BazRequest"
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name:  "single hook, hook reject with code",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					return status.Error(codes.FailedPrecondition, "hook failed")
				}),
			},
			errFunc: wantStatusCode(codes.FailedPrecondition),
		},
		{
			name:  "multi-hook, first hook rejects with code",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					return status.Error(codes.FailedPrecondition, "hook failed")
				}),
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					// never invoked
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.FailedPrecondition),
		},
		{
			name:  "multi-hook, last hook rejects with code",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					return nil
				}),
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					return status.Error(codes.FailedPrecondition, "hook failed")
				}),
			},
			errFunc: wantStatusCode(codes.FailedPrecondition),
		},
		{
			name:  "single hook, hook reject without code",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					return errors.New("non status")
				}),
			},
			errFunc: wantStatusCode(codes.Internal),
		},
		{
			name:  "hook ordering",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					input.MessageType = "Foo.BarRequest"
					return nil
				}),
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					input.MessageType = "Foo.BazRequest"
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name:  "synthesize data, allow",
			input: &RPCAuthInput{Method: "/Foo.Bar/Foo"},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
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
		{
			name:  "synthesize network data, allow",
			input: &RPCAuthInput{Method: "/Foo.Bar/Foo"},
			hooks: []RPCAuthzHook{
				HostNetHook(tcp),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name: "network data allow with justification (no func)",
			input: &RPCAuthInput{
				Method: "/Foo.Bar/Foo",
				Metadata: metadata.MD{
					ReqJustKey: []string{"justification"},
				},
			},
			hooks: []RPCAuthzHook{
				HostNetHook(tcp),
				JustificationHook(nil),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name: "network data allow with justification req but none given (no func)",
			input: &RPCAuthInput{
				Method: "/Foo.Bar/Foo",
			},
			hooks: []RPCAuthzHook{
				HostNetHook(tcp),
				JustificationHook(nil),
			},
			errFunc: wantStatusCode(codes.FailedPrecondition),
		},
		{
			name: "network data allow with justification (with func)",
			input: &RPCAuthInput{
				Method: "/Foo.Bar/Foo",
				Metadata: metadata.MD{
					ReqJustKey: []string{"justification"},
				},
			},
			hooks: []RPCAuthzHook{
				HostNetHook(tcp),
				JustificationHook(func(string) error { return nil }),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name: "network data allow with justification req given and func fails",
			input: &RPCAuthInput{
				Method: "/Foo.Bar/Foo",
				Metadata: metadata.MD{
					ReqJustKey: []string{"justification"},
				},
			},
			hooks: []RPCAuthzHook{
				HostNetHook(tcp),
				JustificationHook(func(string) error { return errors.New("error") }),
			},
			errFunc: wantStatusCode(codes.FailedPrecondition),
		},
		{
			name:  "conditional hook, triggered",
			input: &RPCAuthInput{Method: "/Some.Random/Method"},
			// Set principal to admin if method = "/Some.Random/Method"
			hooks: []RPCAuthzHook{
				HookIf(RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					input.Peer = &PeerAuthInput{
						Principal: &PrincipalAuthInput{
							ID: "admin@foo",
						},
					}
					return nil
				}), func(input *RPCAuthInput) bool {
					return input.Method == "/Some.Random/Method"
				}),
			},
			errFunc: wantStatusCode(codes.OK),
		},
		{
			name:  "conditional hook, not-triggered",
			input: &RPCAuthInput{Method: "/Some.Other/Method"},
			// Set principal to admin if method = "/Some.Random/Method"
			hooks: []RPCAuthzHook{
				HookIf(RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
					input.Peer = &PeerAuthInput{
						Principal: &PrincipalAuthInput{
							ID: "admin@foo",
						},
					}
					return nil
				}), func(input *RPCAuthInput) bool {
					return input.Method == "/Some.Random/Method"
				}),
			},
			errFunc: wantStatusCode(codes.PermissionDenied),
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

func TestNewWithPolicy(t *testing.T) {
	_, err := NewWithPolicy(context.Background(), policyString)
	testutil.FatalOnErr("NewWithPolicy valid", err, t)
	if _, err := NewWithPolicy(context.Background(), ""); err == nil {
		t.Error("didn't get error for empty policy")
	}
}

type testAuthInfo struct {
	credentials.CommonAuthInfo
}

func (testAuthInfo) AuthType() string {
	return "testAuthInfo"
}

func TestRpcAuthInput(t *testing.T) {
	md := metadata.New(map[string]string{
		"foo": "foo",
		"bar": "bar",
	})
	tcp, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:1")
	testutil.FatalOnErr("ResolveIPAddr", err, t)

	for _, tc := range []struct {
		name    string
		ctx     context.Context
		method  string
		req     proto.Message
		compare *RPCAuthInput
	}{
		{
			name:   "method only",
			ctx:    context.Background(),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer:   &PeerAuthInput{},
			},
		},
		{
			name:   "method and metadata",
			ctx:    metadata.NewIncomingContext(context.Background(), md),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method:   "/AMethod",
				Metadata: md,
				Peer:     &PeerAuthInput{},
			},
		},
		{
			name:   "method and request",
			ctx:    context.Background(),
			method: "/AMethod",
			req:    &emptypb.Empty{},
			compare: &RPCAuthInput{
				Method:      "/AMethod",
				Message:     json.RawMessage{0x7b, 0x7d},
				MessageType: "google.protobuf.Empty",
				Peer:        &PeerAuthInput{},
			},
		},
		{
			name:   "method and a peer context but no addr",
			ctx:    peer.NewContext(context.Background(), &peer.Peer{}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer:   &PeerAuthInput{},
			},
		},
		{
			name: "method and a peer context",
			ctx: peer.NewContext(context.Background(), &peer.Peer{
				Addr: tcp,
			}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer: &PeerAuthInput{
					Net: &NetAuthInput{
						Network: "tcp",
						Address: "127.0.0.1",
						Port:    "1",
					},
				},
			},
		},
		{
			name: "method and a peer context with non tls auth",
			ctx: peer.NewContext(context.Background(), &peer.Peer{
				Addr:     tcp,
				AuthInfo: testAuthInfo{},
			}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer: &PeerAuthInput{
					Net: &NetAuthInput{
						Network: "tcp",
						Address: "127.0.0.1",
						Port:    "1",
					},
					Cert: &CertAuthInput{},
				},
			},
		},
		{
			name: "method and a peer context with tls auth",
			ctx: peer.NewContext(context.Background(), &peer.Peer{
				Addr: tcp,
				AuthInfo: credentials.TLSInfo{
					SPIFFEID: &url.URL{
						Path: "/",
					},
					State: tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{
							{}, // Don't need an actual cert, a placeholder will work.
						},
					},
				},
			}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer: &PeerAuthInput{
					Net: &NetAuthInput{
						Network: "tcp",
						Address: "127.0.0.1",
						Port:    "1",
					},
					Cert: &CertAuthInput{SPIFFEID: "/"},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rpcauth, err := NewRPCAuthInput(tc.ctx, tc.method, tc.req)
			testutil.FatalOnErr(tc.name, err, t)
			testutil.DiffErr(tc.name, rpcauth, tc.compare, t)
		})
	}

}

func TestAuthorize(t *testing.T) {
	req := &emptypb.Empty{}
	info := &grpc.UnaryServerInfo{
		FullMethod: "/Foo/Bar",
	}
	gotCalled := false
	handler := func(ctx context.Context, req interface{}) (resp interface{}, err error) {
		gotCalled = true
		return
	}

	authorizer, err := NewWithPolicy(context.Background(), policyString)
	testutil.FatalOnErr("NewWithPolicy", err, t)

	// Should fail on no proto.Message
	_, err = authorizer.Authorize(context.Background(), nil, info, handler)
	testutil.FatalOnNoErr("not a proto message", err, t)

	// Should function normally
	_, err = authorizer.Authorize(context.Background(), req, info, handler)
	testutil.FatalOnErr("Authorize", err, t)
	if !gotCalled {
		t.Fatal("never called handler")
	}

	// Create one with a hook so we can fail.
	authorizer, err = NewWithPolicy(context.Background(), policyString, RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
		return status.Error(codes.FailedPrecondition, "hook failed")
	}))
	testutil.FatalOnErr("NewWithPolicy with hooks", err, t)

	// This time it should fail due to the hook.
	_, err = authorizer.Authorize(context.Background(), req, info, handler)
	testutil.FatalOnNoErr("Authorize with failing hook", err, t)
}

type fakeServerStream struct {
	testutil.FakeServerStream
	Ctx context.Context
}

func (*fakeServerStream) RecvMsg(req interface{}) error {
	return nil
}

func (f *fakeServerStream) Context() context.Context {
	return f.Ctx
}

func TestAuthorizeStream(t *testing.T) {
	req := &emptypb.Empty{}
	info := &grpc.StreamServerInfo{
		FullMethod: "/Foo/Bar",
	}

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return stream.RecvMsg(srv)
	}

	fake := &fakeServerStream{Ctx: context.Background()}

	authorizer, err := NewWithPolicy(context.Background(), policyString)
	testutil.FatalOnErr("NewWithPolicy", err, t)

	err = authorizer.AuthorizeStream(req, fake, info, handler)
	testutil.FatalOnErr("AuthorizeStream", err, t)

	// Should fail with a normal fake server
	err = authorizer.AuthorizeStream(req, &testutil.FakeServerStream{Ctx: context.Background()}, info, handler)
	testutil.FatalOnNoErr("AuthorizeStream failing RecvMsg", err, t)

	// Should fail on a non proto.Message request
	err = authorizer.AuthorizeStream(nil, fake, info, handler)
	testutil.FatalOnNoErr("AuthorizeStream non proto", err, t)

	// Create one with a hook so we can fail.
	authorizer, err = NewWithPolicy(context.Background(), policyString, RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
		return status.Error(codes.FailedPrecondition, "hook failed")
	}))
	testutil.FatalOnErr("NewWithPolicy with hooks", err, t)

	// This time it should fail due to the hook.
	err = authorizer.AuthorizeStream(req, fake, info, handler)
	testutil.FatalOnNoErr("AuthorizeStream with failing hook", err, t)
}
