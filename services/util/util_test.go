package util

import (
	"bytes"
	"context"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func TestRunCommand(t *testing.T) {
	for _, tc := range []struct {
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var opts []Option
			if tc.stderrIsError {
				opts = append(opts, FailOnStderr())
			}

			run, err := RunCommand(context.Background(), tc.bin, tc.args, opts...)
			t.Logf("%s: response: %+v", tc.name, run)
			t.Logf("%s: error: %v", tc.name, err)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("%s: Didn't get error when expected", tc.name)
				}
				return
			}
			if got, want := run.Stdout.String(), tc.stdout; got != want {
				t.Fatalf("%s: Stdout differs. Want %q Got %q", tc.name, want, got)
			}
			if got, want := run.Stderr.String(), tc.stderr; got != want {
				t.Fatalf("%s: Stderr differs. Want %q Got %q", tc.name, want, got)
			}
			if tc.returnCodeNonZero && run.ExitCode == 0 {
				t.Fatalf("%s: Asked for non-zero return code and got 0", tc.name)
			}
		})
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
	for _, tc := range []struct {
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidPath(tc.path)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Errorf("%s: invalid error state. Err %v and got %t and want %t", tc.name, err, got, want)
			}
		})
	}
}

func TestStringSliceFlag(t *testing.T) {
	var flag StringSliceFlag
	if got, want := flag.String(), ""; got != want {
		t.Fatalf("Expected no string from empty flag and got %s", got)
	}
	test := "foo,bar,baz"
	if err := flag.Set(test); err != nil {
		t.Fatalf("error from flag.Set: %v", err)
	}
	if got, want := flag.String(), test; got != want {
		t.Fatalf("flag didn't set to correct value. got %s and want %s", got, want)
	}
	if len(*flag.Target) != 3 {
		t.Fatalf("flag should have 3 elements. Instead is %q", *flag.Target)
	}
}

func TestKeyValueSliceFlag(t *testing.T) {
	var flag KeyValueSliceFlag
	if got, want := flag.String(), ""; got != want {
		t.Fatalf("Expected no string from empty flag and got %s", got)
	}
	test := "foo=bar,baz=bun"
	if err := flag.Set(test); err != nil {
		t.Fatalf("error from flag.Set: %v", err)
	}
	if got, want := flag.String(), test; got != want {
		t.Fatalf("flag didn't set to correct value. got %s and want %s", got, want)
	}
	bad := "foo=bar=baz"
	if err := flag.Set(bad); err == nil {
		t.Fatal("didn't get error from bad flag set as we should")
	}
}

func TestIntSliceFlag(t *testing.T) {
	var flag IntSliceFlags
	if got, want := flag.String(), ""; got != want {
		t.Fatalf("Expected no string from empty flag and got %s", got)
	}
	test := "1,2,3"
	if err := flag.Set(test); err != nil {
		t.Fatalf("error from flag.Set: %v", err)
	}
	if got, want := flag.String(), test; got != want {
		t.Fatalf("flag didn't set to correct value. got %s and want %s", got, want)
	}
	bad := "1,foo,2"
	if err := flag.Set(bad); err == nil {
		t.Fatal("didn't get error from bad flag set as we should")
	}
}
