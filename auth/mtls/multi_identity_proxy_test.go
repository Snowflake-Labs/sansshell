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

package mtls_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	proxyserver "github.com/Snowflake-Labs/sansshell/proxy/server"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	proxytestutil "github.com/Snowflake-Labs/sansshell/proxy/testutil"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

// echoServer is a minimal TestService implementation that echoes the input
// prefixed with the server name. We need our own because
// proxytestutil.EchoTestDataServer.serverName is unexported.
type echoServer struct {
	tdpb.UnimplementedTestServiceServer
	name string
}

func (s *echoServer) TestUnary(_ context.Context, req *tdpb.TestRequest) (*tdpb.TestResponse, error) {
	return &tdpb.TestResponse{Output: fmt.Sprintf("%s %s", s.name, req.Input)}, nil
}

// TestMultiIdentityProxy_E2E verifies the full proxy workflow with
// MultiIdentityCredentials. It starts two target gRPC servers, each
// trusting a different CA, and a proxy that holds both client identities.
// The proxy must select the correct identity for each target or the TLS
// handshake fails.
//
//	Client ──mTLS(Acme)──▶ Proxy ──MultiIdentity──▶ Target A (trusts Acme)
//	                                              ──▶ Target B (trusts Other)
func TestMultiIdentityProxy_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ── target servers ──────────────────────────────────────────────

	targetAAddr := startTargetServer(t, "target-a",
		"testdata/leaf.pem", "testdata/leaf.key", "testdata/root.pem")

	targetBAddr := startTargetServer(t, "target-b",
		"testdata/leaf2.pem", "testdata/leaf2.key", "testdata/root2.pem")

	// ── proxy ───────────────────────────────────────────────────────

	proxyAddr := startProxyServer(t, ctx)

	// ── client → proxy → target A ───────────────────────────────────

	t.Run("route to target A via Acme identity", func(t *testing.T) {
		resp := callThroughProxy(t, ctx, proxyAddr, targetAAddr, "hello")
		if !strings.Contains(resp, "target-a") {
			t.Fatalf("expected response from target-a, got %q", resp)
		}
		if !strings.Contains(resp, "hello") {
			t.Fatalf("expected echo of input, got %q", resp)
		}
	})

	// ── client → proxy → target B ───────────────────────────────────

	t.Run("route to target B via Other identity", func(t *testing.T) {
		resp := callThroughProxy(t, ctx, proxyAddr, targetBAddr, "world")
		if !strings.Contains(resp, "target-b") {
			t.Fatalf("expected response from target-b, got %q", resp)
		}
		if !strings.Contains(resp, "world") {
			t.Fatalf("expected echo of input, got %q", resp)
		}
	})
}

// startTargetServer starts a TLS gRPC server running the TestService echo
// service. The server uses serverCert/serverKey for its TLS identity and
// trusts clientCAFile for client authentication. Returns "host:port".
func startTargetServer(t *testing.T, name, serverCertFile, serverKeyFile, clientCAFile string) string {
	t.Helper()

	clientCA, err := mtls.LoadRootOfTrust(clientCAFile)
	testutil.FatalOnErr("LoadRootOfTrust "+clientCAFile, err, t)

	srvCreds, err := mtls.LoadServerTLS(serverCertFile, serverKeyFile, clientCA)
	testutil.FatalOnErr("LoadServerTLS "+name, err, t)

	lis, err := net.Listen("tcp", "localhost:0")
	testutil.FatalOnErr("Listen "+name, err, t)

	srv := grpc.NewServer(grpc.Creds(srvCreds))
	tdpb.RegisterTestServiceServer(srv, &echoServer{name: name})
	t.Cleanup(srv.GracefulStop)

	go func() { _ = srv.Serve(lis) }()
	return lis.Addr().String()
}

// startProxyServer starts a proxy gRPC server that uses
// MultiIdentityCredentials for outbound connections (Acme + Other client
// identities, merged root CAs) and Acme server creds for incoming
// connections. Returns "host:port".
func startProxyServer(t *testing.T, ctx context.Context) string {
	t.Helper()

	// Incoming (server-side) creds: Acme server cert, trusts Acme client CA.
	clientCA, err := mtls.LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("LoadRootOfTrust proxy server", err, t)

	proxySrvCreds, err := mtls.LoadServerTLS("testdata/leaf.pem", "testdata/leaf.key", clientCA)
	testutil.FatalOnErr("LoadServerTLS proxy", err, t)

	// Outbound (client-side) creds: MultiIdentityCredentials.
	proxyClientCreds := buildMultiIdentityCreds(t)

	authz := proxytestutil.NewAllowAllRPCAuthorizer(ctx, t)

	dialer := proxyserver.NewDialer(grpc.WithTransportCredentials(proxyClientCreds))
	proxySrv := grpc.NewServer(
		grpc.Creds(proxySrvCreds),
		grpc.StreamInterceptor(authz.AuthorizeStream),
	)
	ps := proxyserver.New(dialer, authz)
	ps.Register(proxySrv)
	t.Cleanup(proxySrv.GracefulStop)

	lis, err := net.Listen("tcp", "localhost:0")
	testutil.FatalOnErr("Listen proxy", err, t)

	go func() { _ = proxySrv.Serve(lis) }()
	return lis.Addr().String()
}

// buildMultiIdentityCreds creates a MultiIdentityCredentials with two
// client identities (Acme + Other) and a merged root CA pool. Loaders
// are registered in the global mtls registry.
func buildMultiIdentityCreds(t *testing.T) credentials.TransportCredentials {
	t.Helper()

	registerLoader(t, "test-acme",
		"testdata/client.pem", "testdata/client.key",
		"testdata/leaf.pem", "testdata/leaf.key",
		"testdata/root.pem")
	registerLoader(t, "test-other",
		"testdata/client2.pem", "testdata/client2.key",
		"testdata/leaf2.pem", "testdata/leaf2.key",
		"testdata/root2.pem")
	registerLoader(t, "test-merged-roots",
		"testdata/client.pem", "testdata/client.key",
		"testdata/leaf.pem", "testdata/leaf.key",
		"testdata/roots_merged.pem")

	idAcme, err := mtls.LoadClientIdentity(context.Background(), "test-acme")
	testutil.FatalOnErr("LoadClientIdentity acme", err, t)

	idOther, err := mtls.LoadClientIdentity(context.Background(), "test-other")
	testutil.FatalOnErr("LoadClientIdentity other", err, t)

	logger := testr.New(t)
	creds, err := mtls.NewMultiIdentityCredentials(
		[]mtls.ClientIdentity{idAcme, idOther},
		"test-merged-roots",
		logger,
		nil,
	)
	testutil.FatalOnErr("NewMultiIdentityCredentials", err, t)
	return creds
}

// registerLoader registers a fileLoader in the global mtls registry.
func registerLoader(t *testing.T, name, clientCert, clientKey, serverCert, serverKey, rootCA string) {
	t.Helper()
	err := mtls.Register(name, &fileLoader{
		clientCert: clientCert,
		clientKey:  clientKey,
		serverCert: serverCert,
		serverKey:  serverKey,
		rootCA:     rootCA,
	})
	if err != nil {
		t.Logf("Register %q: %v (may already exist)", name, err)
	}
}

// callThroughProxy connects to the proxy, sends a TestUnary RPC to the
// given target, and returns the response output string.
func callThroughProxy(t *testing.T, ctx context.Context, proxyAddr, targetAddr, input string) string {
	t.Helper()

	clientCA, err := mtls.LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("client LoadRootOfTrust", err, t)

	clientCreds, err := mtls.LoadClientTLS("testdata/client.pem", "testdata/client.key", clientCA)
	testutil.FatalOnErr("LoadClientTLS", err, t)

	conn, err := proxy.DialContext(ctx, proxyAddr, []string{targetAddr},
		grpc.WithTransportCredentials(clientCreds),
	)
	testutil.FatalOnErr(fmt.Sprintf("DialContext proxy→%s", targetAddr), err, t)
	defer conn.Close()

	client := tdpb.NewTestServiceClient(conn)
	resp, err := client.TestUnary(ctx, &tdpb.TestRequest{Input: input})
	testutil.FatalOnErr(fmt.Sprintf("TestUnary to %s", targetAddr), err, t)

	return resp.Output
}

// fileLoader implements mtls.CredentialsLoader using paths to cert files.
type fileLoader struct {
	clientCert, clientKey string
	serverCert, serverKey string
	rootCA                string
}

func (f *fileLoader) LoadClientCertificate(context.Context) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(f.clientCert, f.clientKey)
}

func (f *fileLoader) LoadServerCertificate(context.Context) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(f.serverCert, f.serverKey)
}

func (f *fileLoader) LoadClientCA(context.Context) (*x509.CertPool, error) {
	return mtls.LoadRootOfTrust(f.rootCA)
}

func (f *fileLoader) LoadRootCA(context.Context) (*x509.CertPool, error) {
	return mtls.LoadRootOfTrust(f.rootCA)
}

func (f *fileLoader) CertsRefreshed() bool { return false }

func (f *fileLoader) GetClientCertInfo(context.Context, string) (*mtls.ClientCertInfo, error) {
	return nil, nil
}
