package util

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO(jchacon): Make this configurable
// The chunk size we use when sending replies on a stream.
var StreamingChunkSize = 128 * 1024

type CommandRun struct {
	Stdout   *bytes.Buffer
	Stderr   *bytes.Buffer
	Error    error
	ExitCode int
}

// RunCommand will take the given binary and args and execute it returning all
// relevent state (stdout, stderr, errors, etc).
//
// The binary must be a clean absolute path or an error will result and nothing will
// be run. Any other errors (starting or from waiting) are recorded in the Error field.
// Errors returned directly will be a status.Error and Error will be whatever the exec
// library returns.
func RunCommand(ctx context.Context, bin string, args []string, stderrBad bool) (*CommandRun, error) {
	if !filepath.IsAbs(bin) {
		return nil, status.Errorf(codes.InvalidArgument, "%s is not an absolute path", bin)
	}
	if bin != filepath.Clean(bin) {
		return nil, status.Errorf(codes.InvalidArgument, "%s is not a clean path", bin)
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

	if stderrBad && len(run.Stderr.String()) != 0 {
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
