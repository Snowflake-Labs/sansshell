package mtls

import (
	"context"
	"crypto/x509"
	"log"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Snowflake-Labs/sansshell/server"
	hc "github.com/Snowflake-Labs/sansshell/services/healthcheck"
)

const (
	allowPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "HealthCheck.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
		input.servername = "bufnet"
}
`
	denyPolicy = `
package sansshell.authz

default allow = false

allow {
    input.type = "HealthCheck.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
		input.servername = "localhost"
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
	lis     *bufconn.Listener
	conn    *grpc.ClientConn
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func serverWithPolicy(t *testing.T, policy string, CAPool *x509.CertPool) *grpc.Server {
	t.Helper()
	creds, err := LoadServerTLS("testdata/leaf.pem", "testdata/leaf.key", CAPool)
	if err != nil {
		t.Fatalf("Failed to load client cert: %v", err)
	}
	s, err := server.BuildServer(creds, policy)
	if err != nil {
		t.Fatalf("Could not build server: %s", err)
	}
	listening := make(chan struct{})
	go func() {
		lis = bufconn.Listen(bufSize)
		defer lis.Close()
		listening <- struct{}{}
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	<-listening
	return s
}

func TestHealthCheck(t *testing.T) {
	var err error
	ctx := context.Background()
	CAPool, err := LoadRootOfTrust("testdata/root.pem")
	if err != nil {
		t.Fatalf("Failed to load root CA: %v", err)
	}
	creds, err := LoadClientTLS("testdata/client.pem", "testdata/client.key", CAPool)
	if err != nil {
		t.Fatalf("Failed to load client cert: %v", err)
	}
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
			s := serverWithPolicy(t, tc.Policy, CAPool)
			conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(creds))
			if err != nil {
				t.Fatalf("Failed to dial bufnet: %v", err)
			}
			defer conn.Close()
			client := hc.NewHealthCheckClient(conn)
			resp, err := client.Ok(ctx, &hc.Empty{})
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
