package testutil

import (
	"os/exec"
	"testing"
)

// ResolvePath takes a binary name and attempts to resolve it for testing or fatal out.
func ResolvePath(t *testing.T, path string) string {
	t.Helper()
	binPath, err := exec.LookPath(path)
	if err != nil {
		t.Fatalf("Can't find path for %s: %v", path, err)
	}
	return binPath
}

// FataOnErr is a testing helper function to test and abort on an error.
// Reduces 3 lines to 1 for common error checking.
func FatalOnErr(op string, e error, t *testing.T) {
	t.Helper()
	if e != nil {
		t.Fatalf("%s: err was %v, want nil", op, e)
	}
}
