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

package opa

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func TestNewAuthzPolicy(t *testing.T) {
	expectParseError := func(t *testing.T, err error) {
		if err == nil || !strings.Contains(err.Error(), ast.ParseErr) {
			t.Errorf("%s got error %v, want ParseErr", t.Name(), err)
		}
	}
	expectNoError := func(t *testing.T, err error) {
		testutil.FatalOnErr(t.Name(), err, t)
	}
	for _, tc := range []struct {
		name      string
		policy    string
		query     string
		hintQuery string
		errFunc   func(*testing.T, error)
	}{
		{
			name:    "good minimal policy",
			policy:  "package sansshell.authz",
			errFunc: expectNoError,
		},
		{
			name:    "good minimal policy with alternate query",
			policy:  "package sansshell.authz",
			query:   "data.sansshell.authz.deny",
			errFunc: expectNoError,
		},
		{
			name:   "good policy, bad query",
			policy: "package sansshell.authz",
			query:  "d.sansshell.authz.allow",
			errFunc: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "PrepareForEval") {
					t.Errorf("%s got error %v, want PrepareForEval", t.Name(), err)
				}
			},
		},
		{
			name:      "good minimal policy with hints query",
			policy:    "package sansshell.authz",
			hintQuery: "data.sansshell.authz.denial_hints",
			errFunc:   expectNoError,
		},
		{
			name:    "invalid policy",
			policy:  "foo := bar",
			errFunc: expectParseError,
		},
		{
			name:    "empty policy",
			policy:  "",
			errFunc: expectParseError,
		},
		{
			name:   "non-sansshell package",
			policy: "package another.name",
			errFunc: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "invalid package") {
					t.Errorf("%s got error %v, want error with 'invalid package'", t.Name(), err)
				}
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var opts []Option
			if tc.query != "" {
				opts = append(opts, WithAllowQuery(tc.query))
			}
			if tc.hintQuery != "" {
				opts = append(opts, WithDenialHintsQuery(tc.hintQuery))
			}
			_, err := NewAuthzPolicy(context.Background(), tc.policy, opts...)
			tc.errFunc(t, err)
		})
	}
}

func TestAuthzPolicyEval(t *testing.T) {
	policyString := `
package sansshell.authz

allow {
  input.foo = "bar"
}

allow {
  input.bar = "foo"
}

allow {
  input.foo = "baz"
  input.bar = "bazzle"
}

allow {
  "okayOne" in input.group
}
`
	ctx := context.Background()
	policy, err := NewAuthzPolicy(ctx, policyString)
	testutil.FatalOnErr("NewAuthzPolicy", err, t)

	expectNoError := func(t *testing.T, err error) {
		testutil.FatalOnErr(t.Name(), err, t)
	}

	for _, tc := range []struct {
		name    string
		input   interface{}
		allowed bool
		errFunc func(*testing.T, error)
	}{
		{
			name:    "empty",
			input:   nil,
			allowed: false,
			errFunc: expectNoError,
		},
		{
			name:    "unmatched input",
			input:   map[string]string{"no": "match"},
			allowed: false,
			errFunc: expectNoError,
		},
		{
			name:    "partial match",
			input:   map[string]string{"foo": "baz"},
			allowed: false,
			errFunc: expectNoError,
		},
		{
			name:    "allowed case 1",
			input:   map[string]string{"foo": "bar"},
			allowed: true,
			errFunc: expectNoError,
		},
		{
			name:    "allowed case 2",
			input:   map[string]string{"foo": "baz", "bar": "bazzle"},
			allowed: true,
			errFunc: expectNoError,
		},
		{
			name:    "allowed case 3",
			input:   map[string][]string{"group": {"okayOne", "okayTwo"}},
			allowed: true,
			errFunc: expectNoError,
		},
		{
			name:    "in not matched",
			input:   map[string][]string{"group": {"okayThree", "okayFour"}},
			allowed: false,
			errFunc: expectNoError,
		},
		{
			name:  "input with unmarshalable type",
			input: func() {},
			errFunc: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "policy evaluation") {
					t.Errorf("Eval(), got %v, want err with 'policy evaulation'", err)
				}
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			allowed, err := policy.Eval(ctx, tc.input)
			if allowed != tc.allowed {
				t.Errorf("Eval(), allowed = %v, want %v", allowed, tc.allowed)
			}
			tc.errFunc(t, err)
		})
	}
}

func TestAuthzPolicyDenialHints(t *testing.T) {
	policyString := `
package sansshell.authz

allow {
  input.foo = "baz"
  input.bar = "bazzle"
}

denial_hints[msg] {
	not allow
	msg := "you need to be allowed"
}
denial_hints[msg] {
	input.foo == "baz"
	not input.bar == "bazzle"
	msg := "where is your bazzle"
}
`
	ctx := context.Background()
	policy, err := NewAuthzPolicy(ctx, policyString, WithDenialHintsQuery("data.sansshell.authz.denial_hints"))
	testutil.FatalOnErr("NewAuthzPolicy", err, t)

	for _, tc := range []struct {
		name    string
		input   interface{}
		hints   []string
		errFunc func(*testing.T, error)
	}{
		{
			name:  "empty",
			input: nil,
			hints: []string{"you need to be allowed"},
		},
		{
			name:  "unmatched input",
			input: map[string]string{"no": "match"},
			hints: []string{"you need to be allowed"},
		},
		{
			name:  "partial match",
			input: map[string]string{"foo": "baz"},
			hints: []string{"where is your bazzle", "you need to be allowed"},
		},
		{
			name:  "allowed case",
			input: map[string]string{"foo": "baz", "bar": "bazzle"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			hints, err := policy.DenialHints(ctx, tc.input)
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(hints, tc.hints) {
				t.Errorf("Eval(), allowed = %v, want %v", hints, tc.hints)
			}
		})
	}
}

func TestNewWithPolicy(t *testing.T) {
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

	ctx := context.Background()
	_, err := NewOpaRPCAuthorizer(ctx, policyString)
	testutil.FatalOnErr("NewOpaRPCAuthorizer valid", err, t)
	if _, err := NewOpaRPCAuthorizer(ctx, ""); err == nil {
		t.Error("didn't get error for empty policy")
	}
}
