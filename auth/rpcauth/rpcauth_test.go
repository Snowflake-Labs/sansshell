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
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"

	"go.uber.org/mock/gomock"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

var policyString = `
package sansshell.authz

default allow = false

allow {
  input.method = "/Foo.Bar/Baz"
  input.type = "google.protobuf.Empty"
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

allow {
  some i, j
  input.extensions[i].value = 12345
  input.extensions[j].key = "key1"
}
`

type KeyValExtension struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

type IntExtension struct {
	Value int `json:"value"`
}

func TestAuthzHook(t *testing.T) {
	ctx := context.Background()
	var logs string
	logger := funcr.NewJSON(func(obj string) { logs = obj }, funcr.Options{Verbosity: 2})
	ctx = logr.NewContext(ctx, logger)

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

	// wantLogs asserts that the outputed log message contains the expected fields.
	// If any expected field is not present, it will fail out and print the corresponding error message.
	type expectedLog struct {
		field  string
		errMsg string
	}
	wantLogs := func(expecteds []expectedLog) func(*testing.T, string) {
		return func(t *testing.T, actual string) {
			t.Helper()
			for _, expected := range expecteds {
				if !strings.Contains(actual, expected.field) {
					t.Fatalf("%s. Got %q want %q within it.", expected.errMsg, actual, expected.field)
				}
			}
		}
	}

	extensions, err := json.Marshal([]interface{}{
		&KeyValExtension{
			Key: "key1",
			Val: "val2",
		},
		&IntExtension{
			Value: 12345,
		},
	})
	testutil.FatalOnErr("json.Marshal extensions", err, t)

	for _, tc := range []struct {
		name    string
		input   *RPCAuthInput
		hooks   []RPCAuthzHook
		errFunc func(*testing.T, error)
		logFunc func(*testing.T, string)

		authzMockEval   bool
		authzEvalResult bool
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

			authzMockEval:   true,
			authzEvalResult: false,
		},
		{
			name:  "single hook, create allow",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					input.Message = []byte("{}")
					input.MessageType = "google.protobuf.Empty"
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),

			authzMockEval:   true,
			authzEvalResult: true,
		},
		{
			name:  "extension hook",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
					input.Extensions = extensions
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),

			authzMockEval:   true,
			authzEvalResult: true,
		},
		{
			name:  "multiple hooks, create allow",
			input: &RPCAuthInput{},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					input.Message = []byte("{}")
					input.MessageType = "google.protobuf.Empty"
					return nil
				}),
				RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
					input.Message = []byte("{}")
					input.MessageType = "google.protobuf.Empty"
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),

			authzMockEval:   true,
			authzEvalResult: true,
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
				RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
					input.Method = "/Foo.Bar/Baz"
					input.MessageType = "google.protobuf.Empty"
					input.Message = []byte("{}")
					return nil
				}),
				RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
					input.MessageType = "google.protobuf.Empty"
					input.Message = []byte("{}")
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),

			authzMockEval:   true,
			authzEvalResult: true,
		},
		{
			name:  "synthesize data, allow",
			input: &RPCAuthInput{Method: "/Foo.Bar/Foo"},
			hooks: []RPCAuthzHook{
				RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
					if input.Peer == nil {
						input.Peer = &PeerAuthInput{
							Principal: &PrincipalAuthInput{
								ID:     "admin@foo",
								Groups: []string{"group_bar"},
							},
						}
					}
					return nil
				}),
			},
			errFunc: wantStatusCode(codes.OK),
			logFunc: wantLogs(
				[]expectedLog{
					{
						field:  "\"principal\":{\"id\":\"admin@foo\",\"groups\":[\"group_bar\"]}}",
						errMsg: "this test case should pass the authz hook, so its logs should contain peer principal info which is added by the hook",
					},
					{
						field:  "\"method\":\"/Foo.Bar/Foo\"",
						errMsg: "expected logs to contain request method",
					},
				},
			),

			authzMockEval:   true,
			authzEvalResult: true,
		},
		{
			name:  "synthesize network data, allow",
			input: &RPCAuthInput{Method: "/Foo.Bar/Foo"},
			hooks: []RPCAuthzHook{
				HostNetHook(tcp),
			},
			errFunc: wantStatusCode(codes.OK),

			authzMockEval:   true,
			authzEvalResult: true,
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

			authzMockEval:   true,
			authzEvalResult: true,
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

			authzMockEval:   true,
			authzEvalResult: true,
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
				HookIf(RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
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
			logFunc: wantLogs(
				[]expectedLog{
					{
						field:  "\"principal\":{\"id\":\"admin@foo\",\"groups\":[]}}",
						errMsg: "expected logs to contain peer principal",
					},
					{
						field:  "\"method\":\"/Some.Random/Method\"",
						errMsg: "expected logs to contain /Some.Random/Method method",
					},
				},
			),

			authzMockEval:   true,
			authzEvalResult: true,
		},
		{
			name:  "conditional hook, not-triggered",
			input: &RPCAuthInput{Method: "/Some.Other/Method"},
			// Set principal to admin if method = "/Some.Random/Method"
			hooks: []RPCAuthzHook{
				HookIf(RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
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

			authzMockEval:   true,
			authzEvalResult: false,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			authzPolicyMock := NewMockAuthzPolicy(ctrl)
			if tc.authzMockEval {
				authzPolicyMock.EXPECT().Eval(gomock.Any(), gomock.Any()).Return(tc.authzEvalResult, nil)
				if !tc.authzEvalResult {
					authzPolicyMock.EXPECT().DenialHints(gomock.Any(), gomock.Any()).Return([]string{""}, nil)
				}
			}
			authz := NewRPCAuthorizer(authzPolicyMock, tc.hooks...)
			err := authz.Eval(ctx, tc.input)
			tc.errFunc(t, err)
			if tc.logFunc != nil {
				tc.logFunc(t, logs)
			}
		})
	}
}

func TestAuthorize(t *testing.T) {
	req := &emptypb.Empty{}
	info := &grpc.UnaryServerInfo{
		FullMethod: "/Foo/Bar",
	}
	gotCalled := false
	handler := func(context.Context, interface{}) (interface{}, error) {
		gotCalled = true
		return nil, nil
	}
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	authzPolicyMock := NewMockAuthzPolicy(ctrl)
	authzPolicyMock.EXPECT().Eval(ctx, gomock.Any()).Return(true, nil).Times(1)

	authorizer := NewRPCAuthorizer(authzPolicyMock)

	// Should fail on no proto.Message
	_, err := authorizer.Authorize(ctx, nil, info, handler)
	testutil.FatalOnNoErr("not a proto message", err, t)

	// Should function normally
	_, err = authorizer.Authorize(ctx, req, info, handler)
	testutil.FatalOnErr("Authorize", err, t)
	if !gotCalled {
		t.Fatal("never called handler")
	}

	// Create one with a hook so we can fail.
	authorizer = NewRPCAuthorizer(authzPolicyMock, RPCAuthzHookFunc(func(context.Context, *RPCAuthInput) error {
		return status.Error(codes.FailedPrecondition, "hook failed")
	}))

	// This time it should fail due to the hook.
	_, err = authorizer.Authorize(ctx, req, info, handler)
	testutil.FatalOnNoErr("Authorize with failing hook", err, t)
}

func TestAuthorizeClient(t *testing.T) {
	req := &emptypb.Empty{}
	method := "/Foo/Bar"

	gotCalled := false
	invoker := func(context.Context, string, interface{}, interface{}, *grpc.ClientConn, ...grpc.CallOption) error {
		gotCalled = true
		return nil
	}
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	authzPolicyMock := NewMockAuthzPolicy(ctrl)
	authzPolicyMock.EXPECT().Eval(ctx, gomock.Any()).Return(true, nil).Times(1)

	authorizer := NewRPCAuthorizer(authzPolicyMock)

	// Should fail on no proto.Message
	err := authorizer.AuthorizeClient(ctx, method, nil, nil, nil, invoker)
	testutil.FatalOnNoErr("not a proto message", err, t)

	// Should function normally
	err = authorizer.AuthorizeClient(ctx, method, req, nil, nil, invoker)
	testutil.FatalOnErr("AuthorizeClient", err, t)
	if !gotCalled {
		t.Fatal("never called invoker")
	}

	// Create one with a hook so we can fail.
	authorizer = NewRPCAuthorizer(authzPolicyMock, RPCAuthzHookFunc(func(context.Context, *RPCAuthInput) error {
		return status.Error(codes.FailedPrecondition, "hook failed")
	}))

	// This time it should fail due to the hook.
	err = authorizer.AuthorizeClient(ctx, method, req, nil, nil, invoker)
	testutil.FatalOnNoErr("AuthorizeClient with failing hook", err, t)
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

type fakeClientStream struct {
	testutil.FakeClientStream
}

func (*fakeClientStream) SendMsg(req interface{}) error {
	return nil
}

func TestAuthorizeStream(t *testing.T) {
	req := &emptypb.Empty{}
	info := &grpc.StreamServerInfo{
		FullMethod: "/Foo/Bar",
	}
	ctx := context.Background()

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return stream.RecvMsg(srv)
	}

	fake := &fakeServerStream{Ctx: ctx}

	ctrl := gomock.NewController(t)
	authzPolicyMock := NewMockAuthzPolicy(ctrl)
	authzPolicyMock.EXPECT().Eval(ctx, gomock.Any()).Return(true, nil).Times(1)
	authorizer := NewRPCAuthorizer(authzPolicyMock)

	err := authorizer.AuthorizeStream(req, fake, info, handler)
	testutil.FatalOnErr("AuthorizeStream", err, t)

	// Should fail with a normal fake server
	err = authorizer.AuthorizeStream(req, &testutil.FakeServerStream{Ctx: ctx}, info, handler)
	testutil.FatalOnNoErr("AuthorizeStream failing RecvMsg", err, t)

	// Should fail on a non proto.Message request
	err = authorizer.AuthorizeStream(nil, fake, info, handler)
	testutil.FatalOnNoErr("AuthorizeStream non proto", err, t)

	// Create one with a hook so we can fail.
	authorizer = NewRPCAuthorizer(authzPolicyMock, RPCAuthzHookFunc(func(context.Context, *RPCAuthInput) error {
		return status.Error(codes.FailedPrecondition, "hook failed")
	}))

	// This time it should fail due to the hook.
	err = authorizer.AuthorizeStream(req, fake, info, handler)
	testutil.FatalOnNoErr("AuthorizeStream with failing hook", err, t)
}

func TestAuthorizeClientStream(t *testing.T) {
	req := &emptypb.Empty{}
	method := "/Foo/Bar"
	desc := &grpc.StreamDesc{
		StreamName: "/Bar",
	}

	ctx := context.Background()

	streamer := func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
		return &fakeClientStream{}, nil
	}

	badStreamer := func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
		return nil, errors.New("error")
	}

	ctrl := gomock.NewController(t)
	authzPolicyMock := NewMockAuthzPolicy(ctrl)
	authzPolicyMock.EXPECT().Eval(ctx, gomock.Any()).Return(true, nil).Times(1)
	authorizer := NewRPCAuthorizer(authzPolicyMock)

	// Test bad streamer returns error
	_, err := authorizer.AuthorizeClientStream(ctx, desc, nil, method, badStreamer)
	testutil.FatalOnNoErr("AuthorizeClientStream with bad streamer", err, t)

	stream, err := authorizer.AuthorizeClientStream(ctx, desc, nil, method, streamer)
	testutil.FatalOnErr("AuthorizeClientStream", err, t)

	// Should fail on a non proto.Message request
	err = stream.SendMsg(nil)
	testutil.FatalOnNoErr("AuthorizeClientStream.SendMsg non proto", err, t)

	// Should work with a basic msg
	err = stream.SendMsg(req)
	testutil.FatalOnErr("AuthorizeClientStream real request", err, t)

	// Create one with a hook so we can fail.
	authorizer = NewRPCAuthorizer(authzPolicyMock, RPCAuthzHookFunc(func(context.Context, *RPCAuthInput) error {
		return status.Error(codes.FailedPrecondition, "hook failed")
	}))

	// This time it should fail due to the hook.
	stream, err = authorizer.AuthorizeClientStream(ctx, desc, nil, method, streamer)
	testutil.FatalOnErr("AuthorizeClientStream with failing hook setup", err, t)
	err = stream.SendMsg(req)
	testutil.FatalOnNoErr("AuthorizeClientStream with failing hook", err, t)
}
