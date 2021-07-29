package exec

import (
	"bytes"
	"context"
	"log"
	"net"
	"os"
	"os/exec"
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

func TestExec(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := NewExecClient(conn)
	command := []string{"ls", "-ltr"}
	resp, err := client.Run(ctx, &ExecRequest{Command: command})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	t.Logf("Response: %+v", resp)

	testCmd := exec.CommandContext(ctx, "ls", "-ltr")
	testResp, err := testCmd.CombinedOutput()
	if !bytes.Equal(testResp, resp.GetStdout()) {
		t.Fatalf("contents do not match")
	}
}
