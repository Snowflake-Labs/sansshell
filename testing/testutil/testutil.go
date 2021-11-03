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
