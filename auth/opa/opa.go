package opa

import (
  "context"
  "fmt"
  "log"

  "github.com/open-policy-agent/opa/ast"
  "github.com/open-policy-agent/opa/metrics"
  "github.com/open-policy-agent/opa/rego"
)

const (
  SansshellRegoPackage = "sansshell.authz"
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

// NewAuthzPolicy creates a new AuthzPolicy from a rego policy contained in
// `policy`.
// It returns an error if the policy cannot be parsed, or does not use
// SansshellRegoPackage in its package declaration.
func NewAuthzPolicy(ctx context.Context, policy string) (*AuthzPolicy, error) {
  module, err := ast.ParseModule("sanshell-authz-policy.rego", policy)
  if err != nil {
    return nil, fmt.Errorf("policy parse error: %w", err)
  }

  if !module.Package.Equal(sansshellPackage) {
    return nil, fmt.Errorf("policy has invalid package '%s' (must be '%s')", module.Package, sansshellPackage)
  }

  r := rego.New(
    rego.Query("data.sansshell.authz.allow"),
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
// `input` is permitted.
func (q *AuthzPolicy) Eval(ctx context.Context, input interface{}) (bool, error) {
  evalMetrics := metrics.New()
  defer func() {
    if queryStats, err := evalMetrics.MarshalJSON(); err == nil {
      log.Println("authzPolicy stats:", string(queryStats))
    }
  }()
  results, err := q.query.Eval(ctx, rego.EvalInput(input), rego.EvalMetrics(evalMetrics))
  if err != nil {
    return false, fmt.Errorf("authz policy evaluation error: %w", err)
  }
  return results.Allowed(), nil
}

