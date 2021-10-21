//go:build linux
// +build linux

package server

// OS specific locations for finding test data.
var (
	testdataFile = "./testdata/linux_testdata.textproto"
	testdataPs   = "./testdata/linux.ps"
	badFiles     = []string{
		"./testdata/linux_bad0.ps",
		"./testdata/linux_bad1.ps",
	}
)
