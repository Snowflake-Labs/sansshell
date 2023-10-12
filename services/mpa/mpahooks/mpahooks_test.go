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
	"os"
	"testing"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/opa/proxiedidentity"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	proxyserver "github.com/Snowflake-Labs/sansshell/proxy/server"
	"github.com/Snowflake-Labs/sansshell/services"
	"github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/mpa"
	"github.com/Snowflake-Labs/sansshell/services/mpa/mpahooks"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry"
	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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
	ctx := context.Background()
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
			err := mpahooks.ActionMatchesInput(ctx, tc.action, tc.input)
			if err != nil && tc.matches {
				t.Errorf("expected match: %v", err)
			}
			if err == nil && !tc.matches {
				t.Error("unexpected match")
			}
		})
	}
}

func pollForAction(ctx context.Context, m mpa.MpaClient, method string) (*mpa.Action, error) {
	for {
		l, err := m.List(ctx, &mpa.ListRequest{})
		if err != nil {
			return nil, err
		}
		for _, i := range l.Item {
			if i.Action.Method == method {
				return i.Action, nil
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func clearAll(ctx context.Context, m mpa.MpaClient) error {
	l, err := m.List(ctx, &mpa.ListRequest{})
	if err != nil {
		return err
	}
	for _, i := range l.Item {
		if _, err := m.Clear(ctx, &mpa.ClearRequest{Action: i.Action}); err != nil {
			return err
		}
	}
	return nil
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
	srvAddr := lis.Addr().String()
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
	noInterceptorConn, err := grpc.DialContext(ctx, srvAddr,
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
		conn, err := grpc.DialContext(ctx, srvAddr, grpc.WithTransportCredentials(approverCreds))
		if err != nil {
			return err
		}
		m := mpa.NewMpaClient(conn)

		healthcheckAction, err := pollForAction(ctx, m, "/HealthCheck.HealthCheck/Ok")
		if err != nil {
			return err
		}
		if _, err := m.Approve(ctx, &mpa.ApproveRequest{Action: healthcheckAction}); err != nil {
			return fmt.Errorf("unable to approve %v: %v", healthcheckAction, err)
		}

		fileReadAction, err := pollForAction(ctx, m, "/LocalFile.LocalFile/Read")
		if err != nil {
			return err
		}
		if _, err := m.Approve(ctx, &mpa.ApproveRequest{Action: fileReadAction}); err != nil {
			return fmt.Errorf("unable to approve %v: %v", healthcheckAction, err)
		}
		return nil
	})

	// Make our calls
	conn, err := grpc.DialContext(ctx, srvAddr,
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

	// MPA approvals are stored in a global singleton, so let's clear them
	// to prevent interference with other tests
	if err := clearAll(ctx, mpa.NewMpaClient(noInterceptorConn)); err != nil {
		t.Error(err)
	}
}

var serverBehindProxyPolicy = `
package sansshell.authz

default allow = false

allow {
	input.peer.principal.id = "proxy"
}
`

var proxyPolicy = `
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

allow {
	input.method = "/Proxy.Proxy/Proxy"
}
`

func TestProxiedClientInterceptors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rot, err := mtls.LoadRootOfTrust("../../../auth/mtls/testdata/root.pem")
	if err != nil {
		t.Fatal(err)
	}
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	proxyLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	srvAddr := lis.Addr().String()
	proxyAddr := proxyLis.Addr().String()
	authz, err := rpcauth.NewWithPolicy(ctx, serverBehindProxyPolicy, rpcauth.PeerPrincipalFromCertHook(), mpaserver.ServerMPAAuthzHook())
	if err != nil {
		t.Fatal(err)
	}
	srvCreds, err := mtls.LoadServerTLS("../../../auth/mtls/testdata/leaf.pem", "../../../auth/mtls/testdata/leaf.key", rot)
	if err != nil {
		t.Fatal(err)
	}
	allowProxyToImpersonate := func(ctx context.Context) bool {
		peer := rpcauth.PeerInputFromContext(ctx)
		if peer == nil {
			return false
		}
		return peer.Cert.Subject.CommonName == "proxy"
	}
	s := grpc.NewServer(
		grpc.ChainStreamInterceptor(
			proxiedidentity.ServerProxiedIdentityStreamInterceptor(allowProxyToImpersonate),
			authz.AuthorizeStream,
		),
		grpc.ChainUnaryInterceptor(
			proxiedidentity.ServerProxiedIdentityUnaryInterceptor(allowProxyToImpersonate),
			authz.Authorize,
		),
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

	proxyAuthz, err := rpcauth.NewWithPolicy(ctx, proxyPolicy, rpcauth.PeerPrincipalFromCertHook(), mpahooks.ProxyMPAAuthzHook())
	if err != nil {
		t.Fatal(err)
	}
	proxyCreds, err := mtls.LoadServerTLS("../testdata/proxy.pem", "../testdata/proxy.key", rot)
	if err != nil {
		t.Fatal(err)
	}
	proxyClientCreds, err := mtls.LoadClientTLS("../testdata/proxy.pem", "../testdata/proxy.key", rot)
	if err != nil {
		t.Fatal(err)
	}
	proxySrv := grpc.NewServer(
		grpc.Creds(proxyCreds),
		grpc.ChainUnaryInterceptor(proxyAuthz.Authorize),
		grpc.ChainStreamInterceptor(proxyAuthz.AuthorizeStream),
	)
	proxyserver := proxyserver.New(
		proxyserver.NewDialer(
			grpc.WithTransportCredentials(proxyClientCreds),
			// Telemetry interceptors will forward justification
			grpc.WithChainUnaryInterceptor(telemetry.UnaryClientLogInterceptor(logr.Discard())),
			grpc.WithChainStreamInterceptor(telemetry.StreamClientLogInterceptor(logr.Discard())),
		),
		proxyAuthz,
	)
	proxyserver.Register(proxySrv)
	go func() {
		if err := proxySrv.Serve(proxyLis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer proxySrv.GracefulStop()

	clientCreds, err := mtls.LoadClientTLS("../../../auth/mtls/testdata/client.pem", "../../../auth/mtls/testdata/client.key", rot)
	if err != nil {
		t.Fatal(err)
	}
	approverCreds, err := mtls.LoadClientTLS("../testdata/approver.pem", "../testdata/approver.key", rot)
	if err != nil {
		t.Fatal(err)
	}

	// We test a single target behind the proxy to keep the tests simpler. The underlying
	// MPA code treats a single target as a list of size one, so this should still be
	// representative of MPA across multiple targets.
	var g errgroup.Group
	g.Go(func() error {
		conn, err := proxy.DialContext(ctx, proxyAddr, []string{srvAddr}, grpc.WithTransportCredentials(approverCreds))
		if err != nil {
			return err
		}
		defer conn.Close()
		m := mpa.NewMpaClientProxy(conn)

		healthcheckAction, err := pollForAction(ctx, m, "/HealthCheck.HealthCheck/Ok")
		if err != nil {
			return err
		}
		if _, err := m.Approve(ctx, &mpa.ApproveRequest{Action: healthcheckAction}); err != nil {
			return fmt.Errorf("unable to approve %v: %v", healthcheckAction, err)
		}

		fileReadAction, err := pollForAction(ctx, m, "/LocalFile.LocalFile/Read")
		if err != nil {
			return err
		}
		if _, err := m.Approve(ctx, &mpa.ApproveRequest{Action: fileReadAction}); err != nil {
			return fmt.Errorf("unable to approve %v: %v", healthcheckAction, err)
		}
		return nil
	})

	// Make our calls
	ctx = metadata.AppendToOutgoingContext(ctx, rpcauth.ReqJustKey, "justification")
	conn, err := proxy.DialContext(ctx, proxyAddr, []string{srvAddr},
		grpc.WithTransportCredentials(clientCreds),
		grpc.WithChainStreamInterceptor(mpahooks.StreamClientIntercepter()),
		grpc.WithChainUnaryInterceptor(mpahooks.UnaryClientIntercepter()),
	)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()
	// Confirm that we get Permission Denied before we add the MPA interceptors
	hc := healthcheck.NewHealthCheckClientProxy(conn)
	if _, err := hc.Ok(ctx, &emptypb.Empty{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got something other than permission denied: %v", err)
	}

	state := &util.ExecuteState{
		Conn: conn,
		Out:  []io.Writer{os.Stdout},
		Err:  []io.Writer{os.Stderr},
	}
	conn.StreamInterceptors = []proxy.StreamInterceptor{
		mpahooks.ProxyClientStreamInterceptor(state),
	}
	conn.UnaryInterceptors = []proxy.UnaryInterceptor{
		mpahooks.ProxyClientUnaryInterceptor(state),
	}
	if _, err := hc.Ok(ctx, &emptypb.Empty{}); err != nil {
		t.Error(err)
	}

	file := localfile.NewLocalFileClientProxy(conn)
	bytes, err := file.Read(ctx, &localfile.ReadActionRequest{
		Request: &localfile.ReadActionRequest_File{File: &localfile.ReadRequest{Filename: "/etc/passwd"}},
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
	// MPA approvals are stored in a global singleton, so let's clear them
	// to prevent interference with other tests
	conn.StreamInterceptors = nil
	conn.UnaryInterceptors = nil
	if err := clearAll(ctx, mpa.NewMpaClientProxy(conn)); err != nil {
		t.Error(err)
	}
	fmt.Println("hi")
}
