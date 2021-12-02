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
	"google.golang.org/protobuf/types/known/emptypb"

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
			tu.FatalOnErr("Failed to dial bufnet", err, t)
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
