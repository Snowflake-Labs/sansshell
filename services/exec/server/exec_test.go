package server

import (
	"context"
	"log"
	"net"
	"os"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/exec"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
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
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewExecClient(conn)

	for _, tc := range []struct {
		name              string
		bin               string
		args              []string
		wantErr           bool
		returnCodeNonZero bool
		stdout            string
	}{
		{
			name:   "Basic functionality",
			bin:    testutil.ResolvePath(t, "echo"),
			args:   []string{"hello world"},
			stdout: "hello world\n",
		},
		{
			name:              "Command fails",
			bin:               testutil.ResolvePath(t, "false"),
			returnCodeNonZero: true,
		},
		{
			name:              "Non-existant program",
			bin:               "/something/non-existant",
			returnCodeNonZero: true,
		},
		{
			name:    "non-absolute path",
			bin:     "foo",
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Run(ctx, &pb.ExecRequest{
				Command: tc.bin,
				Args:    tc.args,
			})
			t.Logf("%s: resp: %+v", tc.name, resp)
			t.Logf("%s: err: %v", tc.name, err)
			if tc.wantErr {
				testutil.WantErr(tc.name, err, tc.wantErr, t)
				return
			}
			if got, want := resp.Stdout, tc.stdout; string(got) != want {
				t.Fatalf("%s: stdout doesn't match. Want %q Got %q", tc.name, want, got)
			}
			if got, want := resp.RetCode != 0, tc.returnCodeNonZero; got != want {
				t.Fatalf("%s: Invalid return codes. Non-zero state doesn't match. Want %t Got %t ReturnCode %d", tc.name, want, got, resp.RetCode)
			}
		})
	}
}
