//go:build unix

package server

import (
	"golang.org/x/sys/unix"
)

var (
	osAgnosticRm    = unix.Unlink
	osAgnosticRmdir = unix.Rmdir
)
