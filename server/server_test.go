package server

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	_ "github.com/snowflakedb/unshelled/services/healthcheck"
	lf "github.com/snowflakedb/unshelled/services/localfile"
)

const (
	policy = `
package unshelled.authz

default allow = false

allow {
    input.type = "LocalFile.ReadRequest"
		input.message.filename = "/etc/hosts"
}
allow {
    input.type = "LocalFile.ReadRequest"
		input.message.filename = "/no-such-filename-for-unshelled-unittest"
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

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s, err := BuildServer(lis, policy)
	if err != nil {
		log.Fatalf("Could not build server: %s", err)
	}
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

func TestRead(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	ts := []struct {
		Filename string
		Err      string
	}{
		{
			Filename: "/etc/hosts",
			Err:      "",
		},
		{
			Filename: "/no-such-filename-for-unshelled-unittest",
			Err:      "no such file or directory",
		},
		{
			Filename: "/permission-denied-filename-for-unshelled-unittest",
			Err:      "PermissionDenied",
		},
	}
	for _, want := range ts {
		client := lf.NewLocalFileClient(conn)
		resp, err := client.Read(ctx, &lf.ReadRequest{Filename: want.Filename})
		if err != nil {
			if want.Err == "" {
				t.Errorf("Read failed: %v", err)
			}
			if !strings.Contains(err.Error(), want.Err) {
				t.Errorf("unexpected error; want: %s, got: %s", want.Err, err)
			}
			continue
		}
		t.Logf("Response: %+v", resp)
		contents, err := ioutil.ReadFile(want.Filename)
		if err != nil {
			t.Fatalf("reading test data: %s", err)
		}
		if !bytes.Equal(contents, resp.GetContents()) {
			t.Fatalf("contents do not match")
		}
	}
}
