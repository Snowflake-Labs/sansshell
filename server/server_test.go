package server

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	lfpb "github.com/Snowflake-Labs/sansshell/services/localfile"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/server"
)

const (
	policy = `
package sansshell.authz

default allow = false

allow {
    input.type = "LocalFile.ReadRequest"
		input.message.filename = "/etc/hosts"
}
allow {
    input.type = "LocalFile.ReadRequest"
		input.message.filename = "/no-such-filename-for-sansshell-unittest"
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
	s, err := BuildServer(nil, policy, lis.Addr(), logr.Discard())
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
		t.Fatalf("Failed to dial bufnet: %v", err)
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
			Filename: "/no-such-filename-for-sansshell-unittest",
			Err:      "no such file or directory",
		},
		{
			Filename: "/permission-denied-filename-for-sansshell-unittest",
			Err:      "PermissionDenied",
		},
	}
	for _, want := range ts {
		// Future proof for t.Parallel()
		want := want
		t.Run(want.Filename, func(t *testing.T) {
			client := lfpb.NewLocalFileClient(conn)
			stream, err := client.Read(ctx, &lfpb.ReadRequest{Filename: want.Filename})
			if err != nil {
				// At this point it only returns if we can't connect. Actual errors
				// happen below on the first stream Recv() call.
				if want.Err == "" {
					t.Fatalf("Start of Read failed: %v", err)
				}
			}
			buf := &bytes.Buffer{}
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Logf("Got error: %v", err)
					if want.Err == "" || !strings.Contains(err.Error(), want.Err) {
						t.Errorf("unexpected error; want: %s, got: %s", want.Err, err)
					}
					// If this was an expected error we're done.
					return
				}

				contents := resp.GetContents()
				n, err := buf.Write(contents)
				if got, want := n, len(contents); got != want {
					t.Fatalf("Can't write into buffer at correct length. Got %d want %d", got, want)
				}
				if err != nil {
					t.Fatalf("Can't write into buffer: %v", err)
				}
			}

			contents, err := os.ReadFile(want.Filename)
			if err != nil {
				t.Fatalf("reading test data: %s", err)
			}
			if got, want := buf.Bytes(), contents; !bytes.Equal(got, want) {
				t.Fatalf("contents do not match. Got:\n%s\n\nWant:\n%s", got, want)
			}
		})
	}
}
