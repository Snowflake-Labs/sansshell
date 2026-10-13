/* Copyright (c) 2026 Snowflake Inc. All rights reserved.

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

package server

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func recoverPanic(logger logr.Logger, method string, err *error) {
	if r := recover(); r != nil {
		logger.Error(fmt.Errorf("%v", r), "panic in handler", "method", method, "stack", string(debug.Stack()))
		*err = status.Errorf(codes.Internal, "internal error handling %s", method)
	}
}

func unaryRecoveryInterceptor(logger logr.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer recoverPanic(logger, info.FullMethod, &err)
		return handler(ctx, req)
	}
}

func streamRecoveryInterceptor(logger logr.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer recoverPanic(logger, info.FullMethod, &err)
		return handler(srv, ss)
	}
}
