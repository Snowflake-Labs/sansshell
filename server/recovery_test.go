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
	"testing"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUnaryRecoveryInterceptor(t *testing.T) {
	interceptor := unaryRecoveryInterceptor(logr.Discard())
	info := &grpc.UnaryServerInfo{FullMethod: "/Test.Test/Unary"}

	_, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
		panic("boom")
	})
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("panicking handler: got code %v, want %v (err: %v)", got, codes.Internal, err)
	}

	resp, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
		return "ok", nil
	})
	if err != nil || resp != "ok" {
		t.Fatalf("normal handler: got (%v, %v), want (ok, nil)", resp, err)
	}
}

func TestStreamRecoveryInterceptor(t *testing.T) {
	interceptor := streamRecoveryInterceptor(logr.Discard())
	info := &grpc.StreamServerInfo{FullMethod: "/Test.Test/Stream"}

	err := interceptor(nil, nil, info, func(any, grpc.ServerStream) error {
		panic("boom")
	})
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("panicking handler: got code %v, want %v (err: %v)", got, codes.Internal, err)
	}

	wantErr := status.Error(codes.NotFound, "missing")
	if err := interceptor(nil, nil, info, func(any, grpc.ServerStream) error { return wantErr }); err != wantErr {
		t.Fatalf("erroring handler: got %v, want %v", err, wantErr)
	}
}
