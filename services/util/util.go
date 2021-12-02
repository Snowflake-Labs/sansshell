package util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExecuteState is used by client packages in services to pass
// relevant state down to subcommands.
type ExecuteState struct {
	Conn *proxy.ProxyConn
	Out  []io.Writer
}

// TODO(jchacon): Make this configurable
// The chunk size we use when sending replies on a stream.
var StreamingChunkSize = 128 * 1024

type CommandRun struct {
	Stdout   *bytes.Buffer
	Stderr   *bytes.Buffer
	Error    error
	ExitCode int
}

// There's only one now but compose into a builder pattern for Options so
// it's easy to add more (such as dropping permissions, etc).
type cmdOptions struct {
	failOnStderr bool
}

type Option interface {
	apply(*cmdOptions)
}

type optionfunc func(*cmdOptions)

func (f optionfunc) apply(opts *cmdOptions) {
	f(opts)
}

// If FailOnStderr is passed as am option the command will return an error if any output appears on stderr
// regardless of exit code. As we're often parsing the text output of specific commands as root
// this is a sanity check we're getting expected output. i.e. ps never returns anything on stderr
// so if some run does that's suspect. Up to callers to decide as some tools (yum...) like to emit
// stderr as debugging as they run.
func FailOnStderr() Option {
	return optionfunc(func(o *cmdOptions) {
		o.failOnStderr = true
	})
}

// RunCommand will take the given binary and args and execute it returning all
// relevent state (stdout, stderr, errors, etc).
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

	options := &cmdOptions{}
	for _, opt := range opts {
		opt.apply(options)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	run := &CommandRun{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	// These probably should be streaming through a go-routine to rate limit what we
	// can buffer. In practice output tends to be in the low K range size wise.
	cmd.Stdout = run.Stdout
	cmd.Stderr = run.Stderr
	cmd.Stdin = nil
	// Set to an empty slice to get an empty environment. Nil means inherit.
	cmd.Env = []string{}

	logger.Info("executing local command", "cmd", cmd.String())
	run.Error = cmd.Run()
	run.ExitCode = cmd.ProcessState.ExitCode()

	if options.failOnStderr && len(run.Stderr.String()) != 0 {
		return nil, status.Errorf(codes.Internal, "unexpected error output:\n%s", TrimString(run.Stderr.String()))
	}
	return run, nil
}

// The maximum we should allow stdout or stderr to be when sending back in an error string.
// grpc has limits on how large a returned error can be (generally 4-8k depending on language).
const MAX_BUF = 1024

// TrimString will return the given string truncated to MAX_BUF size so it can be used in
// grpc error replies.
func TrimString(s string) string {
	if len(s) > MAX_BUF {
		s = s[:MAX_BUF]
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

type StringSliceFlag struct {
	Target *[]string
}

func (s *StringSliceFlag) Set(val string) error {
	if s.Target == nil {
		s.Target = new([]string)
	}
	*s.Target = strings.Split(val, ",")
	return nil
}

func (s *StringSliceFlag) String() string {
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

// A type for a custom flag for a list of strings in a comma separated list.
type KeyValueSliceFlag []*KeyValue

// String implements as needed for flag.Value
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

// Set implements parsing for strings list flags as needed
// for flag.Value
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

// A type for a custom flag for a list of ints in a comma separated list.
type IntSliceFlags []int64

// String implements as needed for flag.Value
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

// Set implements parsing for int list flags as needed
// for flag.Value
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
