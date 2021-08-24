package localfile

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net"
	"os"
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

	client := NewLocalFileClient(conn)
	filename := "/etc/hosts"
	resp, err := client.Read(ctx, &ReadRequest{Filename: filename})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	t.Logf("Response: %+v", resp)
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("reading test data: %s", err)
	}
	if !bytes.Equal(contents, resp.GetContents()) {
		t.Fatalf("contents do not match")
	}
}
