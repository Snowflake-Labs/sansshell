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

// Package opa contains code for performing authorization
// checks using opa/rego.
package opa

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	"github.com/go-logr/logr"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
)

const (
	// SansshellRegoPackage is the rego package used by all Sansshell policy files.
	// Any policy not using this package will be rejected.
	SansshellRegoPackage = "sansshell.authz"

	// DefaultAuthzQuery is the default query used for policy evaluation.
	DefaultAuthzQuery = "data.sansshell.authz.allow"
)

var (
	// The Rego package declaration that should be used for sansshell
	// policy files.
	// Attempts to load policy files which do not declare this package
	// will return a helpful error.
	sansshellPackage = ast.MustParsePackage(fmt.Sprintf("package %s", SansshellRegoPackage))
)

// An opaAuthzPolicy performs policy checking by evaluating input against
// a sansshell rego policy file.
type opaAuthzPolicy struct {
	query            rego.PreparedEvalQuery
	denialHintsQuery *rego.PreparedEvalQuery
	b                *bytes.Buffer
}

type policyOptions struct {
	query            string
	denialHintsQuery string
}

// An Option controls the behavior of an opaAuthzPolicy
type Option interface {
	apply(*policyOptions)
}

type optionFunc func(*policyOptions)

func (o optionFunc) apply(opts *policyOptions) {
	o(opts)
}

// WithAllowQuery returns an option to use `query` to evaluate the policy,
// instead of DefaultAuthzQuery. The supplied query should be simple evaluation
// expressions that creates no binding, and evaluates to 'true' iff the input
// satisfies the conditions of the policy.
func WithAllowQuery(query string) Option {
	return optionFunc(func(o *policyOptions) {
		o.query = query
	})
}

// WithDenialHintsQuery returns an option to use `query` to evaluate the policy
// when the AllowPolicy fails. The supplied query must be a simple evaluation
// expression that creates no binding and evaluates to an array of strings.
//
// This can be used to give better error messages when Eval returns false.
// With a value like data.sansshell.authz.denial_hints, you can use a policy
// with rules like
//
//	denial_hints [msg] {
//	  not allow
//	  msg :="you need to be allowed"
//	}
func WithDenialHintsQuery(query string) Option {
	return optionFunc(func(o *policyOptions) {
		o.denialHintsQuery = query
	})
}

// NewOpaAuthzPolicy creates a new opaAuthzPolicy by parsing the policy given
// in the string `policy`.
// It returns an error if the policy cannot be parsed, or does not use
// SansshellRegoPackage in its package declaration.
func NewOpaAuthzPolicy(ctx context.Context, policy string, opts ...Option) (rpcauth.AuthzPolicy, error) {
	options := &policyOptions{
		query: DefaultAuthzQuery,
	}
	for _, opt := range opts {
		opt.apply(options)
	}
	parserOpts := ast.ParserOptions{FutureKeywords: []string{"in"}}
	module, err := ast.ParseModuleWithOpts("sanshell-authz-policy.rego", policy, parserOpts)
	if err != nil {
		return nil, fmt.Errorf("policy parse error: %w", err)
	}

	if !module.Package.Equal(sansshellPackage) {
		return nil, fmt.Errorf("policy has invalid package '%s' (must be '%s')", module.Package, sansshellPackage)
	}

	b := &bytes.Buffer{}
	r := rego.New(
		rego.Query(options.query),
		rego.ParsedModule(module),
		rego.EnablePrintStatements(true),
		rego.PrintHook(topdown.NewPrintHook(b)),
	)

	prepared, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("rego: PrepareForEval() error: %w", err)
	}
	var denialHintsQuery *rego.PreparedEvalQuery
	if options.denialHintsQuery != "" {
		r := rego.New(
			rego.Query(options.denialHintsQuery),
			rego.ParsedModule(module),
		)
		hints, err := r.PrepareForEval(ctx)
		if err != nil {
			return nil, fmt.Errorf("rego: denial hints PrepareForEval() error: %w", err)
		}
		denialHintsQuery = &hints
	}
	return &opaAuthzPolicy{
		query:            prepared,
		denialHintsQuery: denialHintsQuery,
		b:                b,
	}, nil
}

// Eval evaluates this policy using the provided input, returning 'true'
// iff the evaulation was successful, and the operation represented by
// `input` is permitted by the policy.
func (q *opaAuthzPolicy) Eval(ctx context.Context, input *rpcauth.RPCAuthInput) (bool, error) {
	logger := logr.FromContextOrDiscard(ctx)

	var evalInput rego.EvalOption
	if input == nil {
		evalInput = rego.EvalInput(nil)
	} else {
		evalInput = rego.EvalInput(input)
	}

	results, err := q.query.Eval(ctx, evalInput)
	if err != nil {
		return false, fmt.Errorf("authz policy evaluation error: %w", err)
	}
	if q.b.Len() > 0 {
		logger.V(1).Info("print statements", "buffer", q.b.String())
	}
	return results.Allowed(), nil
}

// DenialHints evaluates this policy using the provided input, returning an array
// of strings with reasons for the denial. This is typically used after getting
// a rejection from Eval to give more hints on why the rejection happened.
// It is a no-op if opa.WithDenialHintsQuery was not used.
func (q *opaAuthzPolicy) DenialHints(ctx context.Context, input *rpcauth.RPCAuthInput) ([]string, error) {
	if q.denialHintsQuery == nil {
		return nil, nil
	}
	results, err := q.denialHintsQuery.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, fmt.Errorf("authz policy evaluation error: %w", err)
	}
	if len(results) != 1 {
		return nil, fmt.Errorf("expected exactly one result: %v", results)
	}
	if len(results[0].Bindings) != 0 {
		return nil, fmt.Errorf("too many bindings: %v", results)
	}
	if len(results[0].Expressions) != 1 {
		return nil, fmt.Errorf("expected exactly one expression: %v", results[0].Expressions)
	}
	vals, ok := results[0].Expressions[0].Value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected expression to be an array: %#v", results[0].Expressions[0].Value)
	}
	var hints []string
	for _, v := range vals {
		h, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected expression to be a string: %#v", v)
		}
		hints = append(hints, h)
	}
	sort.Strings(hints)
	return hints, nil
}

// NewOpaRPCAuthorizer creates a new Authorizer from a policy string. Any supplied
// authorization hooks will be executed, in the order provided, on each policy
// evaluation.
// NOTE: The policy is used for both client and server hooks below. If you need
// distinct policy for client vs server, create 2 Authorizer's.
func NewOpaRPCAuthorizer(ctx context.Context, opaPolicy string, authzHooks ...rpcauth.RPCAuthzHook) (rpcauth.RPCAuthorizer, error) {
	opaAuthzPolicy, err := NewOpaAuthzPolicy(ctx, opaPolicy)
	if err != nil {
		return nil, err
	}
	return rpcauth.NewRPCAuthorizer(opaAuthzPolicy, authzHooks...), nil
}
