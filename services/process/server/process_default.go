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
	psBin     = flag.String("ps-bin", "", "Path to the ps binary")
	pstackBin = flag.String("pstack-bin", "", "Path to the pstack binary")
	gcoreBin  = flag.String("gcore-bin", "", "Path to the gcore binary")

	psOptions = func() ([]string, error) {
		return nil, fmt.Errorf("No support for OS %s", runtime.GOOS)
	}
)

func parser(r io.Reader) (map[int64]*ProcessEntry, error) {
	return nil, fmt.Errorf("No support for OS %s", runtime.GOOS)
}
