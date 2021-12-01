package mtls

import (
	"context"
	"crypto/x509"
	"log"
	"net"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Snowflake-Labs/sansshell/server"
	hcpb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	tu "github.com/Snowflake-Labs/sansshell/testing/testutil"
)

const (
	allowPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "HealthCheck.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
    input.peer.net.network = "bufconn"
}

`
	denyPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "HealthCheck.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
    input.peer.net.network = "something else"
}
`
	allowPeerSerialPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "HealthCheck.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
		input.peer.cert.subject.SerialNumber = "255288720161934708870254561641453151839"
}
`
	denyPeerSerialPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "HealthCheck.Empty"
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
	tu.FatalOnErr("Failed to load client cert", err, t)
	lis := bufconn.Listen(bufSize)
	s, err := server.BuildServer(creds, policy, lis.Addr(), logr.Discard())
	tu.FatalOnErr("Could not build server", err, t)
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

func TestHealthCheck(t *testing.T) {
	var err error
	ctx := context.Background()
	CAPool, err := LoadRootOfTrust("testdata/root.pem")
	tu.FatalOnErr("Failed to load root CA", err, t)
	creds, err := LoadClientTLS("testdata/client.pem", "testdata/client.key", CAPool)
	tu.FatalOnErr("Failed to load client cert", err, t)
	ts := []struct {
		Name   string
		Policy string
		Err    string
	}{
		{
			Name:   "allowed request",
			Policy: allowPolicy,
			Err:    "",
		},
		{
			Name:   "denied request",
			Policy: denyPolicy,
			Err:    "OPA policy does not permit this request",
		},
		{
			Name:   "allowed peer by subject serial",
			Policy: allowPeerSerialPolicy,
			Err:    "",
		},
		{
			Name:   "denied peer by subject serial",
			Policy: denyPeerSerialPolicy,
			Err:    "OPA policy does not permit this request",
		},
	}
	for _, tc := range ts {
		t.Run(tc.Name, func(t *testing.T) {
			l, s := serverWithPolicy(t, tc.Policy, CAPool)
			conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer(l)), grpc.WithTransportCredentials(creds))
			tu.FatalOnErr("Failed to dial bufnet", err, t)
			t.Cleanup(func() { conn.Close() })
			client := hcpb.NewHealthCheckClient(conn)
			resp, err := client.Ok(ctx, &hcpb.Empty{})
			if err != nil {
				if tc.Err == "" {
					t.Errorf("Read failed: %v", err)
					return
				}
				if !strings.Contains(err.Error(), tc.Err) {
					t.Errorf("unexpected error; tc: %s, got: %s", tc.Err, err)
				}
				return
			}
			t.Logf("Response: %+v", resp)
			s.GracefulStop()
		})
	}
}
