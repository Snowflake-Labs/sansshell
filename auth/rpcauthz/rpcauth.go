/* Copyright (c) 2025 Snowflake Inc. All rights reserved.

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

package rpcauthz

import (
	"context"
	"google.golang.org/grpc"
)

type RPCAuthorizer interface {
	Eval(ctx context.Context, input *RPCAuthInput) error
	Authorize(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
	AuthorizeClient(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error
	AuthorizeClientStream(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error)
	AuthorizeStream(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
	AppendHooks(hooks ...RPCAuthzHook)
}

// A RPCAuthzHook is invoked on populated RpcAuthInput prior to policy
// evaluation, and may augment / mutate the input, or pre-emptively
// reject a request.
type RPCAuthzHook interface {
	Hook(context.Context, *RPCAuthInput) error
}
