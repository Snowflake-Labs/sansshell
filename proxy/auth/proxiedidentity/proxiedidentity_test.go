/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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

package proxiedidentity

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	healthcheckpb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

type fakeHealthCheck struct {
	callback func(context.Context)
}

func (h *fakeHealthCheck) Ok(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	h.callback(ctx)
	return &emptypb.Empty{}, nil
}

var passingPolicy = `
package sansshell.authz
default authz = false
authz {
  allow
  not deny
}
deny {
  input.metadata["nonexistent-metadata"]
}
allow {
  input.method = "/HealthCheck.HealthCheck/Ok"
}
`

var failingPolicy = `
package sansshell.authz
default authz = false
authz {
  allow
  not deny
}
deny {
  input.metadata["proxied-sansshell-identity"]
}
allow {
  input.method = "/HealthCheck.HealthCheck/Ok"
}
`

func TestProxyingIdentityOverRPC(t *testing.T) {
	ctx := context.Background()

	passingAuthz, err := opa.NewOpaAuthzPolicy(ctx, passingPolicy, opa.WithAllowQuery("data.sansshell.authz.authz"))
	if err != nil {
		t.Fatal(err)
	}
	failingAuthz, err := opa.NewOpaAuthzPolicy(ctx, failingPolicy, opa.WithAllowQuery("data.sansshell.authz.authz"))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		desc            string
		srvInterceptors []grpc.UnaryServerInterceptor
		identityProxied bool
		rpcError        bool
	}{
		{
			desc:            "interceptor missing",
			identityProxied: true,
		},
		{
			desc: "passed",
			srvInterceptors: []grpc.UnaryServerInterceptor{
				ServerProxiedIdentityUnaryInterceptor(),
				rpcauth.NewRPCAuthorizer(passingAuthz).Authorize,
			},
			identityProxied: true,
		},
		{
			desc: "interceptor says no",
			srvInterceptors: []grpc.UnaryServerInterceptor{
				ServerProxiedIdentityUnaryInterceptor(),
				rpcauth.NewRPCAuthorizer(failingAuthz).Authorize,
			},
			rpcError:        true,
			identityProxied: false,
		},
	} {
		ctx := ctx
		t.Run(tc.desc, func(t *testing.T) {

			buffer := 1024
			lis := bufconn.Listen(buffer)
			bufdial := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
			srv := grpc.NewServer(grpc.ChainUnaryInterceptor(tc.srvInterceptors...))
			healthcheck := &fakeHealthCheck{}
			healthcheckpb.RegisterHealthCheckServer(srv, healthcheck)
			go func() {
				if err := srv.Serve(lis); err != nil {
					panic(err)
				}
			}()

			conn, err := grpc.DialContext(ctx, "", grpc.WithContextDialer(bufdial), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatal(err)
			}
			client := healthcheckpb.NewHealthCheckClient(conn)

			identity := &rpcauth.PrincipalAuthInput{
				ID:     "foobar",
				Groups: []string{"baz"},
			}

			ctx = AppendToMetadataInOutgoingContext(ctx, identity)
			var gotIdentity *rpcauth.PrincipalAuthInput
			var gotMetadata []string
			healthcheck.callback = func(ctx context.Context) {
				gotIdentity = FromContext(ctx)
				md, _ := metadata.FromIncomingContext(ctx)
				gotMetadata = md.Get(reqProxiedIdentityKey)
			}
			if _, err := client.Ok(ctx, &emptypb.Empty{}); err != nil {
				if tc.rpcError {
					return
				}
				t.Fatal(err)
			}
			if tc.rpcError {
				t.Error("rpc error was missing")
			}

			if tc.identityProxied {
				if !reflect.DeepEqual(gotIdentity, identity) {
					t.Errorf("got %+v, want %+v", gotIdentity, identity)
				}
			} else {
				if gotIdentity != nil {
					t.Errorf("identity unexpectedly not nil: %+v", gotIdentity)
				}
			}
			if len(gotMetadata) != 1 {
				t.Errorf("expected exactly one metadata val, got %v", gotMetadata)
			} else if gotMetadata[0] != `{"id":"foobar","groups":["baz"]}` {
				t.Errorf("metadata did not match expectation, got %v", gotMetadata[0])
			}
		})
	}
}
