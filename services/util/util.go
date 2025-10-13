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

// Package util provides utility operations used in building sansshell system services.
package util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	writerUtils "github.com/Snowflake-Labs/sansshell/services/util/writer"
	"golang.org/x/term"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
)

// ExecuteState is used by client packages in services to pass
// relevant state down to subcommands.
type ExecuteState struct {
	Conn *proxy.Conn
	// Dir is a directory where additional files per target can be written.
	Dir string
	// Output stream to write log lines related to particular target host.
	Out []io.Writer
	// Error stream to write log lines related to particular target host.
	Err []io.Writer
	// CredSource is the credential source to use for authn.
	CredSource string
}

// StreamingChunkSize is the chunk size we use when sending replies on a stream.
// TODO(jchacon): Make this configurable
var StreamingChunkSize = 128 * 1024

// CommandRun groups all of the status and output from executing a command.
type CommandRun struct {
	Stdout   *LimitedBuffer
	Stderr   *LimitedBuffer
	Error    error
	ExitCode int
}

// A builder pattern for Options so it's easy to add various ones (such as dropping permissions, etc).
type cmdOptions struct {
	failOnStderr bool
	stdoutMax    uint
	stderrMax    uint
	env          []string
	uid          uint32
	gid          uint32
	extraFiles   []*os.File
}

// Option will run the apply operation to change required checking/state
// before executing RunCommand.
type Option interface {
	apply(*cmdOptions)
}

type optionfunc func(*cmdOptions)

func (f optionfunc) apply(opts *cmdOptions) {
	f(opts)
}

// OptionsEqual returns true if the results of applying both Options are equal
func OptionsEqual(a, b Option) bool {
	aCmdOptions := &cmdOptions{}
	bCmdOptions := &cmdOptions{}
	a.apply(aCmdOptions)
	b.apply(bCmdOptions)

	return cmp.Equal(aCmdOptions, bCmdOptions, cmp.AllowUnexported(cmdOptions{}))
}

// OptionsEqual returns true if the results of applying all elements of both Option slices are equal
func OptionSlicesEqual(a, b []Option) bool {
	aCmdOptions := &cmdOptions{}
	bCmdOptions := &cmdOptions{}

	for _, opt := range a {
		opt.apply(aCmdOptions)
	}
	for _, opt := range b {
		opt.apply(bCmdOptions)
	}

	return cmp.Equal(aCmdOptions, bCmdOptions, cmp.AllowUnexported(cmdOptions{}))
}

// FailOnStderr is an option where the command will return an error if any output appears on stderr
// regardless of exit code. As we're often parsing the text output of specific commands as root
// this is a sanity check we're getting expected output. i.e. ps never returns anything on stderr
// so if some run does that's suspect. Up to callers to decide as some tools (yum...) like to emit
// stderr as debugging as they run.
func FailOnStderr() Option {
	return optionfunc(func(o *cmdOptions) {
		o.failOnStderr = true
	})
}

// StdoutMax is an option where the command run will limit output buffered from stdout to this
// many bytes before truncating.
func StdoutMax(max uint) Option {
	return optionfunc(func(o *cmdOptions) {
		o.stdoutMax = max
	})

}

// StderrMax is an option where the command run will limit output buffered from stdout to this
// many bytes before truncating.
func StderrMax(max uint) Option {
	return optionfunc(func(o *cmdOptions) {
		o.stderrMax = max
	})
}

// CommandUser is an option which sets the uid for the Command to run as.
func CommandUser(uid uint32) Option {
	return optionfunc(func(o *cmdOptions) {
		o.uid = uid
	})
}

// CommandGroup is an option which sets the gid for the Command to run as.
func CommandGroup(gid uint32) Option {
	return optionfunc(func(o *cmdOptions) {
		o.gid = gid
	})
}

// EnvVar is an option which sets an environment variable for the sub-processes.
// evar should be of the form foo=bar
func EnvVar(evar string) Option {
	return optionfunc(func(o *cmdOptions) {
		o.env = append(o.env, evar)
	})
}

// ExtraFiles sets file descriptors which should be available in the sub-process.
//
// The table gets passed to exec.Cmd.ExtraFiles, which means that the i-th entry
// in the table, if not nil, will be available in the child process under file
// descriptor number i+3 (0, 1, and 2 are always stdin, stdout, and stderr).
//
// Multiple calls to ExtraFiles will replace the table, not append to it.
func ExtraFiles(files []*os.File) Option {
	return optionfunc(func(o *cmdOptions) {
		o.extraFiles = files
	})
}

// DefRunBufLimit is the default limit we'll buffer for stdout/stderr from RunCommand exec'ing
// a process.
const DefRunBufLimit = 10 * 1024 * 1024

// LimitedBuffer is a bytes.Buffer with the added limitation that it will not grow
// infinitely and will instead stop at a max size limit.
type LimitedBuffer struct {
	max  uint
	full bool
	buf  *bytes.Buffer
}

// NewLimitedBuffer will create a LimitedBuffer with the given maximum size.
func NewLimitedBuffer(max uint) *LimitedBuffer {
	return &LimitedBuffer{
		max: max,
		buf: &bytes.Buffer{},
	}
}

// Write acts exactly as a bytes.Buffer defines except if the underlying
// buffer has reached the max size no more bytes will be added.
// TODO: Implement remaining bytes.Buffer methods if needed.
// NOTE:
//
//	This is not an error condition and instead no more bytes
//	will be written and normal return will happen so writes
//	do not fail. Use the Truncated() method
//	to determine if this has happened.
func (l *LimitedBuffer) Write(p []byte) (int, error) {
	if l.full || uint(len(p)+l.buf.Len()) > l.max {
		if !l.full {
			l.full = true
			// Write enough to fill the buffer and then stop.
			size := int(l.max) - l.buf.Len()
			l.buf.Write(p[:size])
			fmt.Printf("size: %d\n", size)
		}
		// Lie and return the length we could have written.
		return len(p), nil
	}
	return l.buf.Write(p)
}

// String - see bytes.Buffer
func (l *LimitedBuffer) String() string {
	return l.buf.String()
}

// Bytes - see bytes.Buffer
func (l *LimitedBuffer) Bytes() []byte {
	return l.buf.Bytes()
}

// Read - see io.Reader
func (l *LimitedBuffer) Read(p []byte) (int, error) {
	return l.buf.Read(p)
}

// Truncated will return true if the LimitedBuffer has filled and refused
// to write additional bytes.
func (l *LimitedBuffer) Truncated() bool {
	return l.full
}

// RunCommand will take the given binary and args and execute it returning all
// relevent state (stdout, stderr, errors, etc). The returned buffers for stdout
// and stderr are limited by possible options but default limit to DefRunBufLimit.
//
// The binary must be a clean absolute path or an error will result and nothing will
// be run. Any other errors (starting or from waiting) are recorded in the Error field.
// Errors returned directly will be a status.Error and Error will be whatever the exec
// library returns.
func RunCommand(ctx context.Context, bin string, args []string, opts ...Option) (*CommandRun, error) {
	logger := logr.FromContextOrDiscard(ctx)

	if !filepath.IsAbs(bin) {
		return nil, status.Errorf(codes.InvalidArgument, "%s is not an absolute path", bin)
	}
	if bin != filepath.Clean(bin) {
		return nil, status.Errorf(codes.InvalidArgument, "%s is not a clean path", bin)
	}

	euid := uint32(os.Geteuid())
	gid := uint32(os.Getgid())
	options := &cmdOptions{
		stdoutMax: DefRunBufLimit,
		stderrMax: DefRunBufLimit,
		uid:       euid,
		gid:       gid,
	}
	for _, opt := range opts {
		opt.apply(options)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	run := &CommandRun{
		Stdout: NewLimitedBuffer(options.stdoutMax),
		Stderr: NewLimitedBuffer(options.stderrMax),
	}
	// These probably should be streaming through a go-routine to rate limit what we
	// can buffer. In practice output tends to be in the low K range size wise.
	cmd.Stdout = run.Stdout
	cmd.Stderr = run.Stderr
	cmd.Stdin = nil
	// Set to an empty slice to get an empty environment. Nil means inherit.
	cmd.Env = []string{}
	// Now append any we received.
	cmd.Env = append(cmd.Env, options.env...)
	cmd.ExtraFiles = options.extraFiles

	// Set uid/gid if needed for the sub-process to run under.
	// Only do this if it's different than our current ones since
	// attempting to setuid/gid() to even your current values is EPERM.
	if options.uid != euid || options.gid != gid {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{},
		}
		cmd.SysProcAttr.Credential.Uid = options.uid
		cmd.SysProcAttr.Credential.Gid = options.gid
	}
	logger.Info("executing local command", "cmd", cmd.String())
	run.Error = cmd.Run()
	run.ExitCode = cmd.ProcessState.ExitCode()
	// If this was an error it could be two different things. Just exiting non-zero results in an exec.ExitError
	// and we can suppress that as exit code is enough. Otherwise we leave run.Error for callers to use.
	if run.Error != nil {
		if _, ok := run.Error.(*exec.ExitError); ok {
			run.Error = nil
		}
	}
	if options.failOnStderr && len(run.Stderr.String()) != 0 {
		return nil, status.Errorf(codes.Internal, "unexpected error output:\n%s", TrimString(run.Stderr.String()))
	}
	return run, nil
}

// MaxBuf is the maximum we should allow stdout or stderr to be when sending back in an error string.
// grpc has limits on how large a returned error can be (generally 4-8k depending on language).
const MaxBuf = 1024

// TrimString will return the given string truncated to MAX_BUF size so it can be used in
// grpc error replies.
func TrimString(s string) string {
	if len(s) > MaxBuf {
		s = s[:MaxBuf]
	}
	return s
}

// ValidPath ensures the path passed in is both absolute and clean.
func ValidPath(path string) error {
	if !filepath.IsAbs(path) {
		return status.Errorf(codes.InvalidArgument, "%s must be an absolute path", path)
	}
	if path != filepath.Clean(path) {
		return status.Errorf(codes.InvalidArgument, "%s must be a clean path", path)
	}
	return nil
}

// StringSliceFlag is the parsed form of a flag using "foo,bar,baz" style.
type StringSliceFlag struct {
	Target *[]string
}

// Set - see flag.Value
func (s *StringSliceFlag) Set(val string) error {
	if s.Target == nil {
		s.Target = new([]string)
	}
	*s.Target = strings.Split(val, ",")
	return nil
}

// String - see flag.String
func (s *StringSliceFlag) String() string {
	if s.Target == nil {
		return ""
	}
	return strings.Join(*s.Target, ",")
}

// StringSliceCommaOrWhitespaceFlag is the parsed form of a flag accepting
// either "foo,bar,baz" or "foo bar baz" style. This is useful in cases where
// we want to be more flexible in the input values we accept.
type StringSliceCommaOrWhitespaceFlag struct {
	Target *[]string
}

// Set - see flag.Value
func (s *StringSliceCommaOrWhitespaceFlag) Set(val string) error {
	if s.Target == nil {
		s.Target = new([]string)
	}
	for _, vals := range strings.Fields(val) {
		for _, target := range strings.Split(vals, ",") {
			trimmed := strings.TrimSpace(target)
			if trimmed != "" {
				*s.Target = append(*s.Target, trimmed)
			}
		}
	}
	return nil
}

// String - see flag.String
func (s *StringSliceCommaOrWhitespaceFlag) String() string {
	if s.Target == nil {
		return ""
	}
	return strings.Join(*s.Target, ",")
}

// KeyValue is used below with KeyValueSliceFlag to construct foo=bar,baz=foo type of flags.
type KeyValue struct {
	Key   string
	Value string
}

// KeyValueSliceFlag is a custom flag for a list of strings in a comma separated list of the
// form key=value,key=value
type KeyValueSliceFlag []*KeyValue

// String - see flag.Value
func (i *KeyValueSliceFlag) String() string {
	var out bytes.Buffer

	for _, v := range *i {
		out.WriteString(fmt.Sprintf("%s=%s,", v.Key, v.Value))
	}
	o := out.String()
	// Trim last , off the end
	if len(o) > 0 {
		o = o[0 : len(o)-1]
	}
	return o
}

// Set - see flag.Value
func (i *KeyValueSliceFlag) Set(val string) error {
	// Setting will reset anything previous, not append.
	*i = nil
	for _, kv := range strings.Split(val, ",") {
		item := strings.Split(kv, "=")
		if len(item) != 2 {
			return fmt.Errorf("bad key=value: %s", kv)
		}
		*i = append(*i, &KeyValue{
			Key:   item[0],
			Value: item[1],
		})
	}

	return nil
}

// IntSliceFlags is a custom flag for a list of ints in a comma separated list.
type IntSliceFlags []int64

// String - see flag.Value
func (i *IntSliceFlags) String() string {
	var out bytes.Buffer

	for _, t := range *i {
		out.WriteString(fmt.Sprintf("%d,", t))
	}
	o := out.String()
	// Trim last , off the end
	if len(o) > 0 {
		o = o[0 : len(o)-1]
	}
	return o
}

// Set - see flag.Value
func (i *IntSliceFlags) Set(val string) error {
	// Setting will reset anything previous, not append.
	*i = nil
	for _, t := range strings.Split(val, ",") {
		x, err := strconv.ParseInt(t, 0, 64)
		if err != nil {
			return fmt.Errorf("can't parse integer in list: %s", val)
		}
		*i = append(*i, x)
	}
	return nil
}

// IsStreamToTerminal checks if the stream is connected to a terminal
// Could not be covered with test, requires manual testing on changes
func IsStreamToTerminal(stream io.Writer) bool {
	switch v := stream.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	case writerUtils.WrappedWriter:
		return IsStreamToTerminal(v.GetOriginal())
	default:
		return false
	}
}
