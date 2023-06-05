//go:build unix

package server

var readFileTestCases = []struct {
	filename string
	err      string
}{
	{
		filename: "/etc/hosts",
		err:      "",
	},
	{
		filename: "/no-such-filename-for-sansshell-unittest",
		err:      "no such file or directory",
	},
	{
		filename: "/permission-denied-filename-for-sansshell-unittest",
		err:      "PermissionDenied",
	},
}
