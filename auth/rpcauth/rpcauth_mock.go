/*
Copyright (c) 2025 Snowflake Inc. All rights reserved.

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

import "context"

func NewMockAuthzPolicy(authFn func(ctx context.Context, input *RPCAuthInput) (bool, error), denielHintsFn func(ctx context.Context, input *RPCAuthInput) ([]string, error)) AuthzPolicy {
	return &authzPolicyFake{authFn: authFn, denielHintsFn: denielHintsFn}
}

// authzPolicyFake is a mock of AuthzPolicy interface.
type authzPolicyFake struct {
	authFn        func(ctx context.Context, input *RPCAuthInput) (bool, error)
	denielHintsFn func(ctx context.Context, input *RPCAuthInput) ([]string, error)
}

func (a *authzPolicyFake) Eval(ctx context.Context, input *RPCAuthInput) (bool, error) {
	return a.authFn(ctx, input)
}

func (a *authzPolicyFake) DenialHints(ctx context.Context, input *RPCAuthInput) ([]string, error) {
	return a.denielHintsFn(ctx, input)
}
