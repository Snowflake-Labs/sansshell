package localfile

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
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
	s := grpc.NewServer()
	lfs := &server{}
	lfs.Register(s)
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

	tests := []struct {
		Name      string
		Filename  string
		Err       string
		Chunksize int
		Offset    int64
		Length    int64
	}{
		{
			Name:      "/etc/hosts-normal",
			Filename:  "/etc/hosts",
			Err:       "",
			Chunksize: 10,
		},
		{
			Name:      "/etc/hosts-1-byte-chunk",
			Filename:  "/etc/hosts",
			Err:       "",
			Chunksize: 1,
		},
		{
			Name:      "/etc/hosts-with-offset-and-length",
			Filename:  "/etc/hosts",
			Err:       "",
			Chunksize: 10,
			Offset:    10,
			Length:    15,
		},
		{
			Name:      "/etc/hosts-tail",
			Filename:  "/etc/hosts",
			Err:       "",
			Chunksize: 10,
			Offset:    -20,
			Length:    15,
		},
		{
			Name:     "bad-file",
			Filename: "/no-such-filename-for-unshelled-unittest",
			Err:      "no such file or directory",
		},
	}

	for _, tc := range tests {
		// Future proof for t.Parallel()
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			oldChunk := chunkSize
			defer func() {
				chunkSize = oldChunk
			}()

			chunkSize = tc.Chunksize

			client := NewLocalFileClient(conn)

			stream, err := client.Read(ctx, &ReadRequest{
				Filename: tc.Filename,
				Offset:   tc.Offset,
				Length:   tc.Length,
			})
			// In general this can only fail here for connection issues which
			// we're not expecting. Actual failes happen in Recv below.
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}

			buf := &bytes.Buffer{}
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Logf("Got error: %v", err)
					if tc.Err == "" || !strings.Contains(err.Error(), tc.Err) {
						t.Errorf("unexpected error; want: %s, got: %s", tc.Err, err)
					}
					// If this was an expected error we're done.
					return
				}

				t.Logf("Response: %+v", resp)
				contents := resp.GetContents()
				n, err := buf.Write(contents)
				if got, want := n, len(contents); got != want {
					t.Fatalf("Can't write into buffer at correct length. Got %d want %d", got, want)
				}
				if err != nil {
					t.Fatalf("Can't write into buffer: %v", err)
				}
			}
			contents, err := ioutil.ReadFile(tc.Filename)
			if err != nil {
				t.Fatalf("reading test data: %s", err)
			}
			if tc.Offset != 0 || tc.Length != 0 {
				start := 0
				if tc.Offset > 0 {
					start = int(tc.Offset)
				}
				if tc.Offset < 0 {
					start = len(contents) + int(tc.Offset)
				}
				length := 0
				if tc.Length != 0 {
					length = int(tc.Length)
				}
				contents = contents[start : start+length]
			}
			if got, want := buf.Bytes(), contents; !bytes.Equal(got, want) {
				t.Fatalf("contents do not match. Got:\n%s\n\nWant:\n%s", got, want)
			}
		})
	}
}
