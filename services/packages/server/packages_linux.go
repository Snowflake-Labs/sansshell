//go:build linux
// +build linux

package server

import (
	"flag"
)

var yumBin = flag.String("yum_bin", "/usr/bin/yum", "Path to yum binary")
