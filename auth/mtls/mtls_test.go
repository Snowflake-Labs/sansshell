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

package mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/server"
	hcpb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

const (
	allowPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "google.protobuf.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
    input.peer.net.network = "bufconn"
}

`
	denyPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "google.protobuf.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
    input.peer.net.network = "something else"
}
`
	allowPeerSerialPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "google.protobuf.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
		input.peer.cert.subject.SerialNumber = "255288720161934708870254561641453151839"
}
`
	denyPeerSerialPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "google.protobuf.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
		input.peer.cert.subject.SerialNumber = "12345"
}
`
)

var (
	bufSize = 1024 * 1024
)

func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
}

type simpleLoader struct {
	name string
}

func (s *simpleLoader) LoadClientCA(context.Context) (*x509.CertPool, error) {
	if s.name == "errorCA" {
		return nil, errors.New("LoadClientCA error")
	}
	return LoadRootOfTrust("testdata/root.pem")
}

func (s *simpleLoader) LoadRootCA(context.Context) (*x509.CertPool, error) {
	if s.name == "errorCA" {
		return nil, errors.New("LoadRootCA error")
	}
	return LoadRootOfTrust("testdata/root.pem")
}

func (s *simpleLoader) LoadServerCertificate(context.Context) (tls.Certificate, error) {
	if s.name == "errorCert" {
		return tls.Certificate{}, errors.New("LoadServerCertificate error")
	}
	return tls.LoadX509KeyPair("testdata/leaf.pem", "testdata/leaf.key")
}

func (s *simpleLoader) LoadClientCertificate(context.Context) (tls.Certificate, error) {
	if s.name == "errorCert" {
		return tls.Certificate{}, errors.New("LoadClientCertificate error")
	}
	return tls.LoadX509KeyPair("testdata/client.pem", "testdata/client.key")
}

func (s *simpleLoader) CertsRefreshed() bool {
	return s.name == "refresh"
}

func serverWithPolicy(t *testing.T, policy string) (*bufconn.Listener, *grpc.Server) {
	t.Helper()
	Register("refresh", &simpleLoader{name: "refresh"})
	creds, err := LoadServerCredentials(context.Background(), "refresh")
	testutil.FatalOnErr("Failed to load server cert", err, t)
	lis := bufconn.Listen(bufSize)

	s, err := server.BuildServer(
		server.WithCredentials(creds),
		server.WithPolicy(policy),
		server.WithAuthzHook(rpcauth.HostNetHook(lis.Addr())),
	)
	testutil.FatalOnErr("Could not build server", err, t)
	listening := make(chan struct{})
	go func() {
		listening <- struct{}{}
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	<-listening
	return lis, s
}

func TestLoadServerTLS(t *testing.T) {
	CAPool, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("Failed to load root CA", err, t)

	// Make sure this errors if we pass bad data like reversing things.
	_, err = LoadServerTLS("testdata/leaf.key", "testdata/leaf.pem", CAPool)
	t.Log(err)
	testutil.FatalOnNoErr("bad TLS server data", err, t)

	// Also that it works on correct input.
	_, err = LoadServerTLS("testdata/leaf.pem", "testdata/leaf.key", CAPool)
	testutil.FatalOnErr("tls server data", err, t)
}

func TestLoadClientTLS(t *testing.T) {
	CAPool, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("Failed to load root CA", err, t)

	// Make sure this errors if we pass bad data like reversing things.
	_, err = LoadClientTLS("testdata/client.key", "testdata/client.pem", CAPool)
	t.Log(err)
	testutil.FatalOnNoErr("bad TLS client data", err, t)

	// Also that it works on correct input.
	_, err = LoadClientTLS("testdata/client.pem", "testdata/client.key", CAPool)
	testutil.FatalOnErr("tls client data", err, t)
}

func TestLoadRootOfTrust(t *testing.T) {
	_, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("Failed to load root CA", err, t)

	_, err = LoadRootOfTrust("no-file")
	testutil.FatalOnNoErr("bad CA root", err, t)
}

func TestLoadClientServerCredentials(t *testing.T) {
	unregisterAll()
	Register("simple", &simpleLoader{name: "simple"})
	Register("errorCA", &simpleLoader{name: "errorCA"})
	Register("errorCert", &simpleLoader{name: "errorCert"})
	Register("refresh", &simpleLoader{name: "refresh"})

	for _, tc := range []struct {
		name    string
		loader  string
		wantErr bool
	}{
		{
			name:    "bad loader name",
			loader:  "error",
			wantErr: true,
		},
		{
			name:    "bad client CA",
			loader:  "errorCA",
			wantErr: true,
		},
		{
			name:    "bad cert",
			loader:  "errorCert",
			wantErr: true,
		},
		{
			name:   "good creds",
			loader: "simple",
		},
		{
			name:   "refresh creds",
			loader: "refresh",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server, err := LoadServerCredentials(context.Background(), tc.loader)
			testutil.WantErr("server", err, tc.wantErr, t)
			if !tc.wantErr {
				server.OverrideServerName("server")
			}
			client, err := LoadClientCredentials(context.Background(), tc.loader)
			testutil.WantErr("client", err, tc.wantErr, t)
			if !tc.wantErr {
				client.OverrideServerName("server")
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	var err error
	ctx := context.Background()
	unregisterAll()
	Register("refresh", &simpleLoader{name: "refresh"})
	creds, err := LoadClientCredentials(ctx, "refresh")
	creds.OverrideServerName("bufnet")
	testutil.FatalOnErr("Failed to load client cert", err, t)
	for _, tc := range []struct {
		name   string
		policy string
		err    string
	}{
		{
			name:   "allowed request",
			policy: allowPolicy,
			err:    "",
		},
		{
			name:   "denied request",
			policy: denyPolicy,
			err:    "OPA policy does not permit this request",
		},
		{
			name:   "allowed peer by subject serial",
			policy: allowPeerSerialPolicy,
			err:    "",
		},
		{
			name:   "denied peer by subject serial",
			policy: denyPeerSerialPolicy,
			err:    "OPA policy does not permit this request",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			l, s := serverWithPolicy(t, tc.policy)
			conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer(l)), grpc.WithTransportCredentials(creds))
			testutil.FatalOnErr("Failed to dial bufnet", err, t)
			t.Cleanup(func() { conn.Close() })
			client := hcpb.NewHealthCheckClient(conn)
			resp, err := client.Ok(ctx, &emptypb.Empty{})
			if err != nil {
				if tc.err == "" {
					t.Errorf("Read failed: %v", err)
					return
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Errorf("unexpected error; tc: %s, got: %s", tc.err, err)
				}
				return
			}
			t.Logf("Response: %+v", resp)
			s.GracefulStop()
		})
	}
}

type noopLoader struct {
	name string
	CredentialsLoader
}

func TestRegister(t *testing.T) {
	unregisterAll()

	if err := Register("", nil); err == nil {
		t.Error("Didn't get error for nil loader")
	}
	err := Register("foo", noopLoader{name: "foo"})
	testutil.FatalOnErr("Register()", err, t)
	err = Register("bar", noopLoader{name: "bar"})
	testutil.FatalOnErr("Register()", err, t)
	err = Register("foo", noopLoader{name: "foo"})
	testutil.FatalOnNoErr("duplicate entry", err, t)
	testutil.DiffErr("Loaders()", []string{"bar", "foo"}, Loaders(), t)
	for _, name := range []string{"foo", "bar"} {
		l, err := Loader(name)
		testutil.FatalOnErr("Loader()", err, t)
		got := l.(noopLoader).name
		if got != name {
			t.Errorf("Loader(%s) returned loader with name %s, want %s", name, got, name)
		}
	}
}
