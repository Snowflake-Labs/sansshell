//go:build !(linux || darwin)
// +build !linux,!darwin

package process

// OS specific locations for finding test data.
// In this case all blank so tests skip.
var (
	testdataPsFile = ""
	testdataPs     = ""
	badFilesPS     = nil

	testdataPstackNoThreadsFile = ""
	testdataPstackThreadsFile   = ""
)
