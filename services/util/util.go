package util

import (
	"bytes"
	"context"
	"io"
	"log"
	"os/exec"
	"path/filepath"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExecuteState is used by client packages in services to pass
// relevant state down to subcommands.
type ExecuteState struct {
	Conn *proxy.ProxyConn
	Out  io.Writer
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

	log.Printf("Executing: %s", cmd.String())
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
