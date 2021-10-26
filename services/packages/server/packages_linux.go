//go:build linux
// +build linux

package server

import (
	"flag"
)

var yumBin = flag.String("yum-bin", "/usr/bin/yum", "Path to yum binary")
