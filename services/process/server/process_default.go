//go:build !(linux || darwin)
// +build !linux,!darwin

package server

import (
	"flag"
	"fmt"
	"io"
	"runtime"
)

var (
	psBin      = flag.String("ps-bin", "", "Location of the ps command")
	psStackBin = flag.String("pstack-bin", "", "Location of the pstack command")

	psOptions = func() ([]string, error) {
		return nil, fmt.Errorf("No support for OS %s", runtime.GOOS)
	}
)

func parser(r io.Reader) (map[int64]*ProcessEntry, error) {
	return nil, fmt.Errorf("No support for OS %s", runtime.GOOS)
}
