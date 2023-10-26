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

package mpahooks_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/services"
	"github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/mpa"
	"github.com/Snowflake-Labs/sansshell/services/mpa/mpahooks"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/server"
	mpaserver "github.com/Snowflake-Labs/sansshell/services/mpa/server"
)

func mustAny(a *anypb.Any, err error) *anypb.Any {
	if err != nil {
		panic(err)
	}
	return a
}

func TestActionMatchesInput(t *testing.T) {
	for _, tc := range []struct {
		desc    string
		action  *mpa.Action
		input   *rpcauth.RPCAuthInput
		matches bool
	}{
		{
			desc: "basic action",
			action: &mpa.Action{
				User:    "requester",
				Method:  "foobar",
				Message: mustAny(anypb.New(&emptypb.Empty{})),
			},
			input: &rpcauth.RPCAuthInput{
				Method:      "foobar",
				MessageType: "google.protobuf.Empty",
				Message:     []byte("{}"),
				Peer: &rpcauth.PeerAuthInput{
					Principal: &rpcauth.PrincipalAuthInput{
						ID: "requester",
					},
				},
			},
			matches: true,
		},
		{
			desc: "missing auth info",
			action: &mpa.Action{
				User:    "requester",
				Method:  "foobar",
				Message: mustAny(anypb.New(&emptypb.Empty{})),
			},
			input: &rpcauth.RPCAuthInput{
				Method:      "foobar",
				MessageType: "google.protobuf.Empty",
				Message:     []byte("{}"),
			},
			matches: false,
		},
		{
			desc: "wrong message",
			action: &mpa.Action{
				User:    "requester",
				Method:  "foobar",
				Message: mustAny(anypb.New(&mpa.Action{})),
			},
			input: &rpcauth.RPCAuthInput{
				Method:      "foobar",
				MessageType: "google.protobuf.Empty",
				Message:     []byte("{}"),
				Peer: &rpcauth.PeerAuthInput{
					Principal: &rpcauth.PrincipalAuthInput{
						ID: "requester",
					},
				},
			},
			matches: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := mpahooks.ActionMatchesInput(tc.action, tc.input)
			if err != nil && tc.matches {
				t.Errorf("expected match: %v", err)
			}
			if err == nil && !tc.matches {
				t.Error("unexpected match")
			}
		})
	}
}

var serverPolicy = `
package sansshell.authz

default allow = false

allow {
	input.method = "/HealthCheck.HealthCheck/Ok"
	input.peer.principal.id = "sanssh"
	input.approvers[_].id = "approver"
}


allow {
	input.method = "/LocalFile.LocalFile/Read"
	input.peer.principal.id = "sanssh"
	input.approvers[_].id = "approver"
}

allow {
	startswith(input.method, "/Mpa.Mpa/")
}
`

func TestClientInterceptors(t *testing.T) {
	ctx := context.Background()
	rot, err := mtls.LoadRootOfTrust("../../../auth/mtls/testdata/root.pem")
	if err != nil {
		t.Fatal(err)
	}
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	authz, err := rpcauth.NewWithPolicy(ctx, serverPolicy, rpcauth.PeerPrincipalFromCertHook(), mpaserver.ServerMPAAuthzHook())
	if err != nil {
		t.Fatal(err)
	}
	srvCreds, err := mtls.LoadServerTLS("../../../auth/mtls/testdata/leaf.pem", "../../../auth/mtls/testdata/leaf.key", rot)
	if err != nil {
		t.Fatal(err)
	}
	s := grpc.NewServer(
		grpc.ChainStreamInterceptor(authz.AuthorizeStream),
		grpc.ChainUnaryInterceptor(authz.Authorize),
		grpc.Creds(srvCreds),
	)
	for _, svc := range services.ListServices() {
		svc.Register(s)
	}
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	clientCreds, err := mtls.LoadClientTLS("../../../auth/mtls/testdata/client.pem", "../../../auth/mtls/testdata/client.key", rot)
	if err != nil {
		t.Fatal(err)
	}
	approverCreds, err := mtls.LoadClientTLS("../testdata/approver.pem", "../testdata/approver.key", rot)
	if err != nil {
		t.Fatal(err)
	}

	// Confirm that we get Permission Denied without MPA
	noInterceptorConn, err := grpc.DialContext(ctx, lis.Addr().String(),
		grpc.WithTransportCredentials(clientCreds),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := healthcheck.NewHealthCheckClient(noInterceptorConn).Ok(ctx, &emptypb.Empty{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got something other than permission denied: %v", err)
	}
	read, err := localfile.NewLocalFileClient(noInterceptorConn).Read(ctx, &localfile.ReadActionRequest{
		Request: &localfile.ReadActionRequest_File{File: &localfile.ReadRequest{Filename: "/etc/hosts"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := read.Recv(); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got something other than permission denied: %v", err)
	}

	var g errgroup.Group
	g.Go(func() error {
		// Set up an approver loop
		conn, err := grpc.DialContext(ctx, lis.Addr().String(),
			grpc.WithTransportCredentials(approverCreds),
		)
		if err != nil {
			return err
		}
		m := mpa.NewMpaClient(conn)

		var firstItem *mpa.ListResponse_Item
		for {
			l, err := m.List(ctx, &mpa.ListRequest{})
			if err != nil {
				return err
			}
			if len(l.Item) > 1 {
				return fmt.Errorf("too many items: %v", l)
			}
			if len(l.Item) == 1 {
				firstItem = l.Item[0]
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if _, err := m.Approve(ctx, &mpa.ApproveRequest{Action: firstItem.Action}); err != nil {
			return fmt.Errorf("unable to approve %v: %v", firstItem, err)
		}

		var secondItem *mpa.ListResponse_Item
		for {
			l, err := m.List(ctx, &mpa.ListRequest{})
			if err != nil {
				return err
			}
			if len(l.Item) > 2 {
				return fmt.Errorf("too many items: %v", l)
			}
			for _, i := range l.Item {
				if i.Id != firstItem.Id {
					secondItem = i
				}
			}
			if secondItem != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if _, err := m.Approve(ctx, &mpa.ApproveRequest{Action: secondItem.Action}); err != nil {
			return fmt.Errorf("unable to approve %v: %v", secondItem, err)
		}
		return nil
	})

	// Make our calls
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, lis.Addr().String(),
		grpc.WithTransportCredentials(clientCreds),
		grpc.WithChainStreamInterceptor(mpahooks.StreamClientIntercepter()),
		grpc.WithChainUnaryInterceptor(mpahooks.UnaryClientIntercepter()),
	)
	if err != nil {
		t.Error(err)
	}
	hc := healthcheck.NewHealthCheckClient(conn)
	if _, err := hc.Ok(ctx, &emptypb.Empty{}); err != nil {
		t.Error(err)
	}

	file := localfile.NewLocalFileClient(conn)
	bytes, err := file.Read(ctx, &localfile.ReadActionRequest{
		Request: &localfile.ReadActionRequest_File{File: &localfile.ReadRequest{Filename: "/etc/hosts"}},
	})
	if err != nil {
		t.Error(err)
	} else {
		for {
			_, err := bytes.Recv()
			if err != nil {
				if err != io.EOF {
					t.Error(err)
				}
				break
			}
		}
	}

	if err := g.Wait(); err != nil {
		t.Error(err)
	}
}
