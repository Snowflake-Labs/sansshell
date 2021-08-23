package mtls

import (
	"context"
	"crypto/x509"
	"log"
	"net"
	"strings"
	"testing"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/snowflakedb/unshelled/server"
	hc "github.com/snowflakedb/unshelled/services/healthcheck"
)

const (
	allowPolicy = `
package unshelled.authz

default allow = false

allow {
    input.type = "HealthCheck.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
		input.servername = "bufnet"
}
`
	denyPolicy = `
package unshelled.authz

default allow = false

allow {
    input.type = "HealthCheck.Empty"
    input.method = "/HealthCheck.HealthCheck/Ok"
		input.servername = "localhost"
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

func serverWithPolicy(policy string, CAPool *x509.CertPool) *grpc.Server {
	lis = bufconn.Listen(bufSize)
	creds, err := LoadServerTLS("testdata/leaf.pem", "testdata/leaf.key", CAPool)
	if err != nil {
		log.Fatalf("Failed to load client cert: %v", err)
	}
	s, err := server.BuildServer(lis, creds, policy)
	if err != nil {
		log.Fatalf("Could not build server: %s", err)
	}
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	return s
}

func TestHealthCheck(t *testing.T) {
	var err error
	ctx := context.Background()
	CAPool, err := LoadRootOfTrust("testdata/root.pem")
	if err != nil {
		log.Fatalf("Failed to load root CA: %v", err)
	}
	creds, err := LoadClientTLS("testdata/client.pem", "testdata/client.key", CAPool)
	if err != nil {
		log.Fatalf("Failed to load client cert: %v", err)
	}
	ts := []struct {
		Policy string
		Err    string
	}{
		{
			Policy: allowPolicy,
			Err:    "",
		},
		{
			Policy: denyPolicy,
			Err:    "OPA policy does not permit this request",
		},
	}
	for _, want := range ts {
		s := serverWithPolicy(want.Policy, CAPool)
		conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(creds))
		if err != nil {
			log.Fatalf("Failed to dial bufnet: %v", err)
		}
		defer conn.Close()
		client := hc.NewHealthCheckClient(conn)
		resp, err := client.Ok(ctx, &hc.Empty{})
		if err != nil {
			if want.Err == "" {
				t.Errorf("Read failed: %v", err)
				continue
			}
			if !strings.Contains(err.Error(), want.Err) {
				t.Errorf("unexpected error; want: %s, got: %s", want.Err, err)
			}
			continue
		}
		t.Logf("Response: %+v", resp)
		s.GracefulStop()
	}
}
