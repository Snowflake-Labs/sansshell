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

// HostNetHook returns an RpcAuthzHook that sets host networking information.
func HostNetHook(addr net.Addr) RPCAuthzHook {
	return RPCAuthzHookFunc(func(ctx context.Context, input *RPCAuthInput) error {
		if input.Host == nil {
			input.Host = &HostAuthInput{}
		}
		input.Host.Net = NetInputFromAddr(addr)
		return nil
	})
}
