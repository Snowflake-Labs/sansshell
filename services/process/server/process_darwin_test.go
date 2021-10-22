//go:build darwin
// +build darwin

package server

// OS specific locations for finding test data.
var (
	testdataPs   = "./testdata/darwin.ps"
	testdataFile = "./testdata/darwin_testdata.textproto"
	badFiles     = []string{
		"./testdata/darwin_bad0.ps",
		"./testdata/darwin_bad1.ps",
	}
)
