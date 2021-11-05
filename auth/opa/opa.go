// package opa contains code for performing authorization
// checks using opa/rego.
package opa

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

const (
	// The rego package used by all Sansshell policy files.
	// Any policy not using this package will be rejected.
	SansshellRegoPackage = "sansshell.authz"

	// The default query used to evalate the policy
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

// Use `query` to evaulate the policy, instead of DefaultAuthzQuery
// The supplied query should be simple evaluation expression that
// defaults to true if the input satisfies the policy, and
// should create no bindings.
func WithAllowQuery(query string) Option {
	return optionFunc(func(o *policyOptions) {
		o.query = query
	})
}

// NewAuthzPolicy creates a new AuthzPolicy from a rego policy contained in
// `policy`.
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
