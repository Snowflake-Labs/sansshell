//go:build !linux
// +build !linux

package server

import (
	"flag"
)

var yumBin = flag.String("yum_bin", "false", "Path to yum binary (NOTE: no support on this platform)")
