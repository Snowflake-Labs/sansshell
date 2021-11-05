package opa

import (
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestNewAuthzPolicy(t *testing.T) {
	expectParseError := func(t *testing.T, err error) {
		if err == nil || !strings.Contains(err.Error(), ast.ParseErr) {
			t.Errorf("%s got error %v, want ParseErr", t.Name(), err)
		}
	}
	expectNoError := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("%s got error %v, want nil", t.Name(), err)
		}
	}
	for _, tc := range []struct {
		name    string
		policy  string
		errFunc func(*testing.T, error)
	}{
		{
			name:    "good minimal policy",
			policy:  `package sansshell.authz`,
			errFunc: expectNoError,
		},
		{
			name:    "invalid policy",
			policy:  `foo := bar`,
			errFunc: expectParseError,
		},
		{
			name:    "empty policy",
			policy:  "",
			errFunc: expectParseError,
		},
		{
			name:   "non-sansshell package",
			policy: `package another.name`,
			errFunc: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "invalid package") {
					t.Errorf("%s got error %v, want error with 'invalid package'", t.Name(), err)
				}
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAuthzPolicy(context.Background(), tc.policy)
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
`
	ctx := context.Background()
	policy, err := NewAuthzPolicy(ctx, policyString)
	if err != nil {
		t.Fatalf("NewAuthzPolicy, err was %v, want nil", err)
	}

	expectNoError := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("%s got error %v, want nil", t.Name(), err)
		}
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
