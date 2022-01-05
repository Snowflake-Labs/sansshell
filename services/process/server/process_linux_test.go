//go:build linux
// +build linux

package server

// OS specific locations for finding test data.
var (
	testdataPsTextProto = "./testdata/linux_testdata.ps.textproto"
	testdataPs          = "./testdata/linux.ps"
	badFilesPs          = []string{
		"./testdata/linux_bad0.ps", // empty file
		"./testdata/linux_bad1.ps", // bad pid
		"./testdata/linux_bad2.ps", // bad field count
		"./testdata/linux_bad3.ps", // bad nice value
	}

	testdataPstackNoThreads              = "./testdata/linux_pstack_no_threads.txt"
	testdataPstackNoThreadsTextProto     = "./testdata/linux_pstack_no_threads.textproto"
	testdataPstackThreads                = "./testdata/linux_pstack_threads.txt"
	testdataPstackThreadsTextProto       = "./testdata/linux_pstack_threads.textproto"
	testdataPstackThreadsBadThread       = "./testdata/linux_pstack_threads_bad_thread.txt"
	testdataPstackThreadsBadThreadNumber = "./testdata/linux_pstack_threads_bad_thread_number.txt"
	testdataPstackThreadsBadThreadId     = "./testdata/linux_pstack_threads_bad_thread_id.txt"
	testdataPstackThreadsBadLwp          = "./testdata/linux_pstack_threads_bad_lwp.txt"
)
