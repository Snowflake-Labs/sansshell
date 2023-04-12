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
	"net"

	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Metrics
const (
	authzDeniedJustificationMetricName = "authz_denied_justification"
)

// RPCAuthzHookFunc implements RpcAuthzHook for a simple function
type RPCAuthzHookFunc func(context.Context, *RPCAuthInput) error

// Hook runs the hook function on the given input
func (r RPCAuthzHookFunc) Hook(ctx context.Context, input *RPCAuthInput) error {
	return r(ctx, input)
}

// A HookPredicate returns true if a conditional hook should run
type HookPredicate func(*RPCAuthInput) bool

// HookIf wraps an existing hook, and only executes it when
// the provided condition returns true
func HookIf(hook RPCAuthzHook, condition HookPredicate) RPCAuthzHook {
	return &conditionalHook{
		hook:      hook,
		predicate: condition,
	}
}

type conditionalHook struct {
	hook      RPCAuthzHook
	predicate HookPredicate
}

func (c *conditionalHook) Hook(ctx context.Context, input *RPCAuthInput) error {
	if c.predicate(input) {
		return c.hook.Hook(ctx, input)
	}
	return nil
}

// HostNetHook returns an RPCAuthzHook that sets host networking information.
func HostNetHook(addr net.Addr) RPCAuthzHook {
	return RPCAuthzHookFunc(func(_ context.Context, input *RPCAuthInput) error {
		if input.Host == nil {
			input.Host = &HostAuthInput{}
		}
		input.Host.Net = NetInputFromAddr(addr)
		return nil
	})
}

const (
	// ReqJustKey is the key name that must exist in the incoming
	// context metadata if client side provided justification is required.
	ReqJustKey = "sansshell-justification"
)

var (
	// ErrJustification is the error returned for missing justification.
	ErrJustification = status.Error(codes.FailedPrecondition, "missing justification")
)

// JustificationHook takes the given optional justification function and returns an RPCAuthzHook
// that validates if justification was included. If it is required and passes the optional validation function
// it will return nil, otherwise an error.
func JustificationHook(justificationFunc func(string) error) RPCAuthzHook {
	return RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
		// See if we got any metadata and if it contains the justification
		var j string
		v := input.Metadata[ReqJustKey]
		if len(v) > 0 {
			j = v[0]
		}
		logger := logr.FromContextOrDiscard(ctx)
		if j == "" {
			if metrics.Enabled() {
				errRegister := metrics.RegisterInt64Counter(authzDeniedJustificationMetricName, "authorization denied due to justification")
				if errRegister != nil {
					logger.V(1).Error(errRegister, "failed to register "+authzDeniedJustificationMetricName)
				}
				errCounter := metrics.AddCount(ctx, authzDeniedJustificationMetricName, 1)
				if errCounter != nil {
					logger.V(1).Error(errCounter, "failed to add counter "+authzDeniedJustificationMetricName)
				}
			}
			return ErrJustification
		}
		if justificationFunc != nil {
			if err := justificationFunc(j); err != nil {
				if metrics.Enabled() {
					errRegister := metrics.RegisterInt64Counter(authzDeniedJustificationMetricName, "authorization denied due to justification")
					if errRegister != nil {
						logger.V(1).Error(errRegister, "failed to register "+authzDeniedJustificationMetricName)
					}
					errCounter := metrics.AddCount(ctx, authzDeniedJustificationMetricName, 1)
					if errCounter != nil {
						logger.V(1).Error(errCounter, "failed to add counter "+authzDeniedJustificationMetricName)
					}
				}
				return status.Errorf(codes.FailedPrecondition, "justification failed: %v", err)
			}
		}
		return nil
	})
}
