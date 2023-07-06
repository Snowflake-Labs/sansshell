/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

package util

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func TestRunCommand(t *testing.T) {
	for _, tc := range []struct {
		name              string
		bin               string
		args              []string
		wantErr           bool
		wantRunErr        bool
		returnCodeNonZero bool
		uid               uint32
		gid               uint32
		stdout            string
		stderr            string
		stderrIsError     bool
		env               []string
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
			wantRunErr:        true,
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
			name:   "verify env vars",
			bin:    testutil.ResolvePath(t, "env"),
			stdout: "FOO=bar\nBAZ=e\n",
			env:    []string{"FOO=bar", "BAZ=e"},
		},
		{
			name:              "error codes",
			bin:               testutil.ResolvePath(t, "false"),
			returnCodeNonZero: true,
		},
		{
			name:              "set uid (will fail)",
			bin:               testutil.ResolvePath(t, "env"),
			uid:               99,
			returnCodeNonZero: true,
			wantRunErr:        true,
		},
		{
			name:              "set gid (will fail)",
			bin:               testutil.ResolvePath(t, "env"),
			gid:               99,
			returnCodeNonZero: true,
			wantRunErr:        true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			opts := []Option{
				StdoutMax(100),
				StderrMax(100),
			}
			if tc.stderrIsError {
				opts = append(opts, FailOnStderr())
			}
			for _, e := range tc.env {
				opts = append(opts, EnvVar(e))
			}
			if tc.uid != 0 {
				opts = append(opts, CommandUser(tc.uid))
			}
			if tc.gid != 0 {
				opts = append(opts, CommandGroup(tc.gid))
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
			testutil.WantErr(tc.name, run.Error, tc.wantRunErr, t)
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
	for i := 0; i < 2*MaxBuf; i++ {
		b.WriteByte('c')
	}
	w := TrimString(b.String())
	if got, want := w, b.String(); got == want {
		t.Fatalf("TrimString didn't trim string. Got:\n%q\nWant:\n%q\n", got, want)
	}
}

func TestLimitedBuffer(t *testing.T) {
	for _, tc := range []struct {
		name        string
		max         uint
		numBytes    int
		isTruncated bool
	}{
		{
			name:        "basic functionality",
			max:         10,
			numBytes:    20,
			isTruncated: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			lb := NewLimitedBuffer(tc.max)
			buf := make([]byte, tc.numBytes)
			for i := 0; i < tc.numBytes; i++ {
				buf[i] = 'a'
			}
			// Do this twice so we know we filled it potentially.
			for i := 0; i < 2; i++ {
				n, err := lb.Write(buf)
				t.Logf("n: %d err: %v", n, err)
				testutil.FatalOnErr(tc.name, err, t)
				if got, want := n, tc.numBytes; got != want {
					t.Fatalf("didn't get expected byte count back. got %d want %d", got, want)
				}
			}
			if got, want := len(lb.String()), int(tc.max); got != want {
				t.Fatalf("wrote too much to buffer. wrote %d want %d - buffer %q", got, want, lb.String())
			}
			if got, want := len(lb.Bytes()), int(tc.max); got != want {
				t.Fatalf("wrote too much to buffer. wrote %d want %d", got, want)
			}
			n, err := lb.Read(buf)
			testutil.FatalOnErr(tc.name, err, t)
			if got, want := n, int(tc.max); got != want {
				t.Fatalf("Reading from buf got wrong result. Want %d and got %d", want, got)
			}
			if got, want := tc.isTruncated, lb.Truncated(); got != want {
				t.Fatalf("truncate state bad. Want %t got %t", want, got)
			}
		})
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

func TestStringSliceCommaOrWhitespaceFlag(t *testing.T) {
	for _, tc := range []struct {
		desc       string
		input      string
		wantTarget []string
		wantString string
	}{
		{
			desc: "empty input",
		},
		{
			desc:       "simple case",
			input:      "foo,bar,baz",
			wantTarget: []string{"foo", "bar", "baz"},
			wantString: "foo,bar,baz",
		},
		{
			desc:       "space separated",
			input:      "foo bar baz",
			wantTarget: []string{"foo", "bar", "baz"},
			wantString: "foo,bar,baz",
		},
		{
			desc: "newline separated",
			input: `foo
bar
baz`,
			wantTarget: []string{"foo", "bar", "baz"},
			wantString: "foo,bar,baz",
		},
		{
			desc:       "space and comma separated",
			input:      "foo,bar baz",
			wantTarget: []string{"foo", "bar", "baz"},
			wantString: "foo,bar,baz",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var flag StringSliceCommaOrWhitespaceFlag
			if err := flag.Set(tc.input); err != nil {
				t.Errorf("error from flag.Set: %v", err)
			}
			if got := flag.String(); got != tc.wantString {
				t.Errorf("flag didn't set to correct value. got %s and want %s", got, tc.wantString)
			}
			if !reflect.DeepEqual(*flag.Target, tc.wantTarget) {
				t.Errorf("flag parsed as %v, wanted %v", *flag.Target, tc.wantTarget)
			}
		})
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

func TestOptionsSliceEqual(t *testing.T) {
	for _, tc := range []struct {
		optionA        []Option
		optionB        []Option
		expectedResult bool
		name           string
	}{
		{
			name:           "multiple EnvVars",
			optionA:        []Option{EnvVar("A=A"), EnvVar("B=B")},
			optionB:        []Option{EnvVar("A=A"), EnvVar("B=B")},
			expectedResult: true,
		},
		{
			name:           "should be true because FailOnStdErr is idempotent",
			optionA:        []Option{FailOnStderr(), FailOnStderr()},
			optionB:        []Option{FailOnStderr()},
			expectedResult: true,
		},
		{
			name:           "Ordering of idempotent funcs shouldn't matter",
			optionA:        []Option{StdoutMax(3), EnvVar("A=A")},
			optionB:        []Option{EnvVar("A=A"), StdoutMax(3)},
			expectedResult: true,
		},
		{
			name:           "EnvVar ordering does matter",
			optionA:        []Option{EnvVar("A=A"), EnvVar("B=B")},
			optionB:        []Option{EnvVar("B=B"), EnvVar("A=A")},
			expectedResult: false,
		},
		{
			optionA:        []Option{CommandGroup(1)},
			optionB:        []Option{CommandUser(2), EnvVar("FOO=BAR")},
			expectedResult: false,
		},
	} {
		tc := tc
		result := OptionSlicesEqual(tc.optionA, tc.optionB)
		if result != tc.expectedResult {
			t.Fatalf("test %s failed: expected %v, got %v", tc.name, tc.expectedResult, result)
		}
	}
}

func TestOptionsEqual(t *testing.T) {
	for _, tc := range []struct {
		optionA        Option
		optionB        Option
		expectedResult bool
	}{
		{
			optionA:        EnvVar(""),
			optionB:        EnvVar(""),
			expectedResult: true,
		},
		{
			optionA:        FailOnStderr(),
			optionB:        FailOnStderr(),
			expectedResult: true,
		},
		{
			optionA:        EnvVar("SANSSHELL_PROXY=sansshell.com"),
			optionB:        EnvVar("SANSSHELL_PROXY=sansshell.com"),
			expectedResult: true,
		},
		{
			optionA:        EnvVar(""),
			optionB:        EnvVar("FOO=false"),
			expectedResult: false,
		},
		{
			optionA:        CommandGroup(1),
			optionB:        CommandUser(2),
			expectedResult: false,
		},
	} {
		tc := tc
		result := OptionsEqual(tc.optionA, tc.optionB)
		if result != tc.expectedResult {
			t.Fatalf("expected %v, got %v", tc.expectedResult, result)
		}
	}
}
