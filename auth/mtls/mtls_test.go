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

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

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

func serverWithPolicy(t *testing.T, policy string, CAPool *x509.CertPool) (*bufconn.Listener, *grpc.Server) {
	t.Helper()
	creds, err := LoadServerTLS("testdata/leaf.pem", "testdata/leaf.key", CAPool)
	testutil.FatalOnErr("Failed to load client cert", err, t)
	lis := bufconn.Listen(bufSize)
	s, err := server.BuildServer(creds, policy, lis.Addr(), logr.Discard())
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
	if err == nil {
		t.Fatal("didn't get an error for bad TLS server data")
	}
}

func TestLoadClientTLS(t *testing.T) {
	CAPool, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("Failed to load root CA", err, t)

	// Make sure this errors if we pass bad data like reversing things.
	_, err = LoadClientTLS("testdata/leaf.key", "testdata/leaf.pem", CAPool)
	t.Log(err)
	if err == nil {
		t.Fatal("didn't get an error for bad TLS client data")
	}
}

func TestLoadRootOfTrust(t *testing.T) {
	_, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("Failed to load root CA", err, t)

	if _, err := LoadRootOfTrust("no-file"); err == nil {
		t.Fatal("didn't get error for bad root CA")
	}
}

type simpleLoader struct {
	name string
	CredentialsLoader
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
	return tls.LoadX509KeyPair("testdata/leaf.pem", "testdata/leaf.key")
}

func TestLoadClientServerCredentials(t *testing.T) {
	unregisterAll()
	Register("simple", &simpleLoader{name: "simple"})
	Register("errorCA", &simpleLoader{name: "errorCA"})
	Register("errorCert", &simpleLoader{name: "errorCert"})

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
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadServerCredentials(context.Background(), tc.loader)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Fatalf("server: didn't get expected error state. got %t want %t err %v", got, want, err)
			}
			_, err = LoadClientCredentials(context.Background(), tc.loader)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Fatalf("client: didn't get expected error state. got %t want %t err %v", got, want, err)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	var err error
	ctx := context.Background()
	CAPool, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("Failed to load root CA", err, t)
	creds, err := LoadClientTLS("testdata/client.pem", "testdata/client.key", CAPool)
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
			l, s := serverWithPolicy(t, tc.policy, CAPool)
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
	if err := Register("foo", noopLoader{name: "foo"}); err == nil {
		t.Error("Didn't get error for duplicate entry")
	}
	if diff := cmp.Diff([]string{"bar", "foo"}, Loaders()); diff != "" {
		t.Errorf("Loaders() mismatch (-want, +got):\n%s", diff)
	}
	for _, name := range []string{"foo", "bar"} {
		l, err := Loader(name)
		testutil.FatalOnErr("Loader()", err, t)
		got := l.(noopLoader).name
		if got != name {
			t.Errorf("Loader(%s) returned loader with name %s, want %s", name, got, name)
		}
	}
}
