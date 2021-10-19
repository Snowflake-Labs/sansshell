package ansible

// NOTE: This doesn't run a real ansible-playbook binary as that would require build hosts to have
//       ansible installed which is a bit much. Instead everything is faked to validate the right
//       options are passed, output/return values come back across, etc.
//
//       If you want a local integration test use testdata/test.yml with a built client/server to prove the real
//       binary works as well.
import (
	"context"
	"log"
	"net"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestRun(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedAnsiblePlaybookBin := *ansiblePlaybookBin

	savedCmdArgsTransform := cmdArgsTransform
	cmdArgsTransform = func(input []string) []string {
		// Initially so cat will run and exit. Below
		// it'll be replaced to look for specific args.
		return []string{"/dev/null"}
	}
	defer func() {
		*ansiblePlaybookBin = savedAnsiblePlaybookBin
		cmdArgsTransform = savedCmdArgsTransform
	}()

	client := NewPlaybookClient(conn)

	// Test 0: A bad command that doesn't exec.
	*ansiblePlaybookBin = "/non-existant-command"
	resp, err := client.Run(ctx, &RunRequest{Playbook: "/PLAYBOOK"})
	if err == nil {
		t.Fatalf("Expected error for bad command. Instead got: +%v", resp)
	}
	t.Log(err)

	// Test 1: A command that exits non-zero
	*ansiblePlaybookBin = "false"
	resp, err = client.Run(ctx, &RunRequest{Playbook: "/PLAYBOOK"})
	if err != nil {
		t.Fatalf("Unexpected error for non-zero exiting command. %v", err)
	}
	t.Log(err)

	if resp.ReturnCode == 0 {
		t.Fatalf("Got a 0 return code from false. Reply: %+v", resp)
	}

	// Test 1: Validate stdout/stderr have expected data.
	*ansiblePlaybookBin = "sh"
	cmdArgsTransform = func(input []string) []string {
		return []string{
			"-c",
			"echo foo >&2 && echo bar",
		}
	}
	resp, err = client.Run(ctx, &RunRequest{Playbook: "/PLAYBOOK"})
	if err != nil {
		t.Fatalf("Unexpected error for stdout/stderr test. %v", err)
	}
	if got, want := resp.Stdout, "bar\n"; got != want {
		t.Errorf("Wrong stdout output. Got %q Want %q", got, want)
	}
	if got, want := resp.Stderr, "foo\n"; got != want {
		t.Errorf("Wrong stderr output. Got %q Want %q", got, want)
	}

	// Reset for remaining tests
	*ansiblePlaybookBin = "cat"

	// Test 2: Run without a path set.
	resp, err = client.Run(ctx, &RunRequest{})
	if err == nil {
		t.Fatalf("Expected error for no path. Instead got: +%v", resp)
	}
	t.Log(err)

	// Test 3: Run without an absolute path
	resp, err = client.Run(ctx, &RunRequest{Playbook: "some_path"})
	if err == nil {
		t.Fatalf("Expected error for not being an absolute path path. Instead got: +%v", resp)
	}
	t.Log(err)

	baseArgs := []string{
		"-i",
		"localhost,",
		"--connection=local",
	}

	// Test 4: Table driven test of various arg combos.
	//         Playbook arg is the same for all and added below
	//         at the top of test logic each time.
	for _, test := range []struct {
		name     string
		wantArgs []string
		req      *RunRequest
	}{
		{
			name:     "single playbook",
			wantArgs: baseArgs,
			req:      &RunRequest{},
		},
		{
			name: "extra vars",
			wantArgs: append(baseArgs, []string{
				"-e",
				"foo=bar",
				"-e",
				"baz=baz",
			}...),
			req: &RunRequest{
				Vars: []*Var{
					{Key: "foo", Value: "bar"},
					{Key: "baz", Value: "baz"},
				},
			},
		},
		{
			name: "become",
			wantArgs: append(baseArgs, []string{
				"--become",
				"USER",
			}...),
			req: &RunRequest{
				User: "USER",
			},
		},
		{
			name:     "check",
			wantArgs: append(baseArgs, "--check"),
			req: &RunRequest{
				Check: true,
			},
		},
		{
			name:     "diff",
			wantArgs: append(baseArgs, "--diff"),
			req: &RunRequest{
				Diff: true,
			},
		},
		{
			name:     "verbose",
			wantArgs: append(baseArgs, "-vvv"),
			req: &RunRequest{
				Verbose: true,
			},
		},
	} {
		// Things every request does the same.
		test.req.Playbook = "/PLAYBOOK"
		test.wantArgs = append(test.wantArgs, "/PLAYBOOK")

		var savedArgs []string
		diff := ""
		cmdArgsTransform = func(input []string) []string {
			savedArgs = input
			diff = cmp.Diff(input, test.wantArgs)
			return []string{"/dev/null"}
		}
		resp, err = client.Run(ctx, test.req)
		if err != nil {
			t.Fatalf("Unexpected error checking %s: %v", test.name, err)
		}
		if diff != "" {
			t.Fatalf("Different args for %s\nDiff:\n%s\nGot:\n%q\nWant:\n%q", test.name, diff, savedArgs, test.wantArgs)
		}
		t.Log(resp)
	}
}
