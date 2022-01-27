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
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
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

// An AuthzPolicy performs policy checking by evaluating input against
// a sansshell rego policy file.
type AuthzPolicy struct {
	query rego.PreparedEvalQuery
}

type policyOptions struct {
	query string
}

// An Option controls the behavior of an AuthzPolicy
type Option interface {
	apply(*policyOptions)
}

type optionFunc func(*policyOptions)

func (o optionFunc) apply(opts *policyOptions) {
	o(opts)
}

// WithAllowQuery returns an option to use `query` to evaulate the policy,
// instead of DefaultAuthzQuery. The supplied query should be simple evaluation
// expressions that creates no binding, and evaluates to 'true' iff the input
// satisfies the conditions of the policy.
func WithAllowQuery(query string) Option {
	return optionFunc(func(o *policyOptions) {
		o.query = query
	})
}

// NewAuthzPolicy creates a new AuthzPolicy by parsing the policy given
// in the string `policy`.
// It returns an error if the policy cannot be parsed, or does not use
// SansshellRegoPackage in its package declaration.
func NewAuthzPolicy(ctx context.Context, policy string, opts ...Option) (*AuthzPolicy, error) {
	options := &policyOptions{
		query: DefaultAuthzQuery,
	}
	for _, opt := range opts {
		opt.apply(options)
	}
	module, err := ast.ParseModule("sanshell-authz-policy.rego", policy)
	if err != nil {
		return nil, fmt.Errorf("policy parse error: %w", err)
	}

	if !module.Package.Equal(sansshellPackage) {
		return nil, fmt.Errorf("policy has invalid package '%s' (must be '%s')", module.Package, sansshellPackage)
	}

	r := rego.New(
		rego.Query(options.query),
		rego.ParsedModule(module),
	)

	prepared, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("rego: PrepareForEval() error: %w", err)
	}
	return &AuthzPolicy{query: prepared}, nil
}

// Eval evaluates this policy using the provided input, returning 'true'
// iff the evaulation was successful, and the operation represented by
// `input` is permitted by the policy.
func (q *AuthzPolicy) Eval(ctx context.Context, input interface{}) (bool, error) {
	results, err := q.query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return false, fmt.Errorf("authz policy evaluation error: %w", err)
	}
	return results.Allowed(), nil
}
