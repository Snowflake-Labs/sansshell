//go:build windows

package server

var readFileTestCases = []struct {
	filename string
	err      string
}{
	{
		filename: "C:\\Windows\\win.ini",
		err:      "",
	},
	{
		filename: "C:\\no-such-filename-for-sansshell-unittest",
		err:      "The system cannot find the file specified",
	},
	{
		filename: "/permission-denied-filename-for-sansshell-unittest",
		err:      "PermissionDenied",
	},
}
