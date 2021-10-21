package exec

import (
	"bytes"
	"context"
	"log"
	"net"
	"os"
	"os/exec"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/exec"
	"google.golang.org/grpc"
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
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewExecClient(conn)

	// Test 0: Basic functionality
	command := []string{"echo", "hello world"}
	resp, err := client.Run(ctx, &pb.ExecRequest{Command: command[0], Args: command[1:]})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	t.Logf("Response: %+v", resp)

	testCmd := exec.CommandContext(ctx, "echo", "hello world")
	testResp, err := testCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Exec for echo failed: %v", err)
	}
	if !bytes.Equal(testResp, resp.GetStdout()) {
		t.Fatalf("contents do not match")
	}

	// Test 1: Execute false so the command fails but RPC should not.
	command = []string{"false"}
	resp, err = client.Run(ctx, &pb.ExecRequest{Command: command[0], Args: command[1:]})
	if err != nil {
		t.Fatalf("Exec for false failed: %v", err)
	}
	t.Logf("Response: %+v", resp)

	if got, want := resp.RetCode, int32(1); got != want {
		t.Fatalf("Wrong response codes for %q. Got %d want %d", command[0], got, want)
	}

	// Test 2: Non-existant program.
	command = []string{"/something/non-existant"}
	resp, err = client.Run(ctx, &pb.ExecRequest{Command: command[0], Args: command[1:]})
	if err == nil {
		t.Fatalf("Expected failure for %q. Got %v", command[0], resp)
	}
	t.Logf("Response: %+v", resp)
}
