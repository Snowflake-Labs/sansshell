//go:build unix

package server

var (
	nonAbsolutePath = "/tmp/foo/../../etc/passwd"
	validFile       = "/etc/hosts"
)
