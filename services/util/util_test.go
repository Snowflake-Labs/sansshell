package util

import (
	"bytes"
	"context"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func TestRunCommand(t *testing.T) {
	for _, test := range []struct {
		name              string
		bin               string
		args              []string
		wantErr           bool
		returnCodeNonZero bool
		stdout            string
		stderr            string
		stderrIsError     bool
	}{
		{
			name:    "Not absolute path",
			bin:     "false",
			wantErr: true,
		},
		{
			name:    "Not clean path",
			bin:     testutil.ResolvePath(t, "false") + "../",
			wantErr: true,
		},
		{
			name:              "non-existant binary",
			bin:               "/non-existant-path",
			returnCodeNonZero: true,
		},
		{
			name:   "Command with stdout and stderr",
			bin:    testutil.ResolvePath(t, "sh"),
			args:   []string{"-c", "echo foo >&2 && echo bar"},
			stdout: "bar\n",
			stderr: "foo\n",
		},
		{
			name:          "Command with stdout and stderr but stderr is error",
			bin:           testutil.ResolvePath(t, "sh"),
			args:          []string{"-c", "echo foo >&2 && echo bar"},
			stdout:        "bar\n",
			stderr:        "foo\n",
			stderrIsError: true,
			wantErr:       true,
		},
		{
			name:   "Verify clean environment",
			bin:    testutil.ResolvePath(t, "env"),
			stdout: "",
		},
		{
			name:              "error codes",
			bin:               testutil.ResolvePath(t, "false"),
			returnCodeNonZero: true,
		},
	} {
		var opts []Option
		if test.stderrIsError {
			opts = append(opts, FailOnStderr())
		}

		run, err := RunCommand(context.Background(), test.bin, test.args, opts...)
		t.Logf("%s: response: %+v", test.name, run)
		t.Logf("%s: error: %v", test.name, err)
		if test.wantErr {
			if err == nil {
				t.Fatalf("%s: Didn't get error when expected", test.name)
			}
			continue
		}
		if got, want := run.Stdout.String(), test.stdout; got != want {
			t.Fatalf("%s: Stdout differs. Want %q Got %q", test.name, want, got)
		}
		if got, want := run.Stderr.String(), test.stderr; got != want {
			t.Fatalf("%s: Stderr differs. Want %q Got %q", test.name, want, got)
		}
		if test.returnCodeNonZero && run.ExitCode == 0 {
			t.Fatalf("%s: Asked for non-zero return code and got 0", test.name)
		}
	}
}

func TestTrimString(t *testing.T) {
	b := &bytes.Buffer{}
	for i := 0; i < 2*MAX_BUF; i++ {
		b.WriteByte('c')
	}
	w := TrimString(b.String())
	if got, want := w, b.String(); got == want {
		t.Fatalf("TrimString didn't trim string. Got:\n%q\nWant:\n%q\n", got, want)
	}
}

func TestValidPath(t *testing.T) {
	for _, test := range []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name: "valid path",
			path: "/",
		},
		{
			name:    "Non absolute path",
			path:    "../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "Non clean path",
			path:    "/tmp/../tmp",
			wantErr: true,
		},
	} {
		err := ValidPath(test.path)
		if got, want := err != nil, test.wantErr; got != want {
			t.Errorf("%s: invalid error state. Err %v and got %t and want %t", test.name, err, got, want)
		}
	}
}
