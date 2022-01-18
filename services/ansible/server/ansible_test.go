package server

// NOTE: This doesn't run a real ansible-playbook binary as that would require build hosts to have
//       ansible installed which is a bit much. Instead everything is faked to validate the right
//       options are passed, output/return values come back across, etc.
//
//       If you want a local integration test use testdata/test.yml with a built client/server to prove the real
//       binary works as well. i.e. what testing/integrate.sh does.
import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/ansible"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func TestRun(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedAnsiblePlaybookBin := *ansiblePlaybookBin

	savedCmdArgsTransform := cmdArgsTransform
	cmdArgsTransform = func(input []string) []string {
		// Initially so cat will run and exit. Below
		// it'll be replaced to look for specific args.
		return []string{"/dev/null"}
	}
	t.Cleanup(func() {
		*ansiblePlaybookBin = savedAnsiblePlaybookBin
		cmdArgsTransform = savedCmdArgsTransform
	})

	client := pb.NewPlaybookClient(conn)

	wd, err := os.Getwd()
	testutil.FatalOnErr("can't get current working directory", err, t)

	path := filepath.Join(wd, "testdata", "test.yml")

	for _, tc := range []struct {
		name              string
		bin               string
		path              string
		args              []string
		user              string
		vars              []*pb.Var
		wantErr           bool
		returnCodeNonZero bool
		stdout            string
		stderr            string
	}{
		{
			name:    "A non-absolute bin path",
			bin:     "something",
			path:    path,
			wantErr: true,
		},
		{
			name:              "A bad command that doesn't exec",
			bin:               "/non-existant-command",
			path:              path,
			returnCodeNonZero: true,
		},
		{
			name:              "A command that exits non-zero",
			bin:               testutil.ResolvePath(t, "false"),
			path:              path,
			returnCodeNonZero: true,
		},
		{
			name:   "Validate stdout/stderr have expected data",
			bin:    testutil.ResolvePath(t, "sh"),
			path:   path,
			args:   []string{"-c", "echo foo >&2 && echo bar"},
			stdout: "bar\n",
			stderr: "foo\n",
		},
		{
			name:    "Run without a path set",
			bin:     testutil.ResolvePath(t, "cat"),
			wantErr: true,
		},
		{
			name:    "Run without an absolute path",
			bin:     testutil.ResolvePath(t, "cat"),
			path:    "some_path",
			wantErr: true,
		},
		{
			name:    "Run with an absolute path but points to a directory",
			bin:     testutil.ResolvePath(t, "cat"),
			path:    "/",
			wantErr: true,
		},
		{
			name:    "absolute path but appends some additional items that would be bad",
			bin:     testutil.ResolvePath(t, "cat"),
			path:    path + " && rm -rf /",
			wantErr: true,
		},
		{
			name:    "Badly named user",
			bin:     testutil.ResolvePath(t, "cat"),
			path:    path,
			user:    "user && rm -rf /",
			wantErr: true,
		},
		{
			name: "Bad Key",
			bin:  testutil.ResolvePath(t, "cat"),
			path: path,
			vars: []*pb.Var{
				{
					Key:   "key && rm -rf /",
					Value: "val",
				},
			},
			wantErr: true,
		}, {
			name: "Bad Value",
			bin:  testutil.ResolvePath(t, "cat"),
			path: path,
			vars: []*pb.Var{
				{
					Key:   "key",
					Value: "val && rm -rf /",
				},
			},
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			*ansiblePlaybookBin = tc.bin
			cmdArgsTransform = func(input []string) []string {
				return tc.args
			}
			resp, err := client.Run(ctx, &pb.RunRequest{
				Playbook: tc.path,
				User:     tc.user,
				Vars:     tc.vars,
			})
			t.Logf("%s: resp: %+v", tc.name, resp)
			t.Logf("%s: err: %v", tc.name, err)
			if tc.wantErr {
				testutil.WantErr(tc.name, err, tc.wantErr, t)
				return
			}
			if tc.returnCodeNonZero && resp.ReturnCode == 0 {
				t.Fatalf("%s: Invalid return codes. Wanted non-zero and got zero", tc.name)
			}
			if got, want := resp.Stdout, tc.stdout; got != want {
				t.Fatalf("%s: Stdout doesn't match. Want %q Got %q", tc.name, want, got)
			}
			if got, want := resp.Stderr, tc.stderr; got != want {
				t.Fatalf("%s: Stderr doesn't match. Want %q Got %q", tc.name, want, got)
			}
		})
	}

	// Table driven test of various arg combos.
	// Playbook arg is the same for all and added below
	// at the top of test logic each time.
	baseArgs := []string{
		"-i",
		"localhost,",
		"--connection=local",
	}

	for _, tc := range []struct {
		name     string
		wantArgs []string
		req      *pb.RunRequest
	}{
		{
			name:     "single playbook",
			wantArgs: baseArgs,
			req:      &pb.RunRequest{},
		},
		{
			name: "extra vars",
			wantArgs: append(baseArgs, []string{
				"-e",
				"foo=bar",
				"-e",
				"baz=BAZ0_",
			}...),
			req: &pb.RunRequest{
				Vars: []*pb.Var{
					{Key: "foo", Value: "bar"},
					{Key: "baz", Value: "BAZ0_"},
				},
			},
		},
		{
			name: "become",
			wantArgs: append(baseArgs, []string{
				"--become",
				"USER",
			}...),
			req: &pb.RunRequest{
				User: "USER",
			},
		},
		{
			name:     "check",
			wantArgs: append(baseArgs, "--check"),
			req: &pb.RunRequest{
				Check: true,
			},
		},
		{
			name:     "diff",
			wantArgs: append(baseArgs, "--diff"),
			req: &pb.RunRequest{
				Diff: true,
			},
		},
		{
			name:     "verbose",
			wantArgs: append(baseArgs, "-vvv"),
			req: &pb.RunRequest{
				Verbose: true,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Things every request does the same.
			tc.req.Playbook = path
			tc.wantArgs = append(tc.wantArgs, path)

			var savedArgs []string
			diff := ""
			cmdArgsTransform = func(input []string) []string {
				savedArgs = input
				diff = cmp.Diff(input, tc.wantArgs)
				return []string{"/dev/null"}
			}
			resp, err := client.Run(ctx, tc.req)
			testutil.FatalOnErr("unexpected error", err, t)
			if diff != "" {
				t.Fatalf("Different args for %s\nDiff:\n%s\nGot:\n%q\nWant:\n%q", tc.name, diff, savedArgs, tc.wantArgs)
			}
			t.Log(resp)
		})
	}
}
