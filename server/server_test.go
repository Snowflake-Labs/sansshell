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
	tu "github.com/Snowflake-Labs/sansshell/testing/testutil"
)

const (
	policy = `
package sansshell.authz

default allow = false

allow {
    input.type = "LocalFile.ReadActionRequest"
		input.message.file.filename = "/etc/hosts"
}
allow {
    input.type = "LocalFile.ReadActionRequest"
		input.message.file.filename = "/no-such-filename-for-sansshell-unittest"
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
	tu.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	for _, tc := range []struct {
		filename string
		err      string
	}{
		{
			filename: "/etc/hosts",
			err:      "",
		},
		{
			filename: "/no-such-filename-for-sansshell-unittest",
			err:      "no such file or directory",
		},
		{
			filename: "/permission-denied-filename-for-sansshell-unittest",
			err:      "PermissionDenied",
		},
	} {
		tc := tc
		t.Run(tc.filename, func(t *testing.T) {
			client := lfpb.NewLocalFileClient(conn)
			stream, err := client.Read(ctx, &lfpb.ReadActionRequest{
				Request: &lfpb.ReadActionRequest_File{
					File: &lfpb.ReadRequest{
						Filename: tc.filename,
					},
				},
			})
			if err != nil {
				// At this point it only returns if we can't connect. Actual errors
				// happen below on the first stream Recv() call.
				if tc.err == "" {
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
					if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
						t.Errorf("unexpected error; want: %s, got: %s", tc.err, err)
					}
					// If this was an expected error we're done.
					return
				}

				contents := resp.GetContents()
				n, err := buf.Write(contents)
				if got, want := n, len(contents); got != want {
					t.Fatalf("Can't write into buffer at correct length. Got %d want %d", got, want)
				}
				tu.FatalOnErr("Can't write into buffer", err, t)
			}

			contents, err := os.ReadFile(tc.filename)
			tu.FatalOnErr("reading test data", err, t)
			if got, want := buf.Bytes(), contents; !bytes.Equal(got, want) {
				t.Fatalf("contents do not match. Got:\n%s\n\nWant:\n%s", got, want)
			}
		})
	}
}
