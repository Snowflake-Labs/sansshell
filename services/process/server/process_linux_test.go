//go:build linux
// +build linux

/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

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
	testdataPstackThreadsBadThreadID     = "./testdata/linux_pstack_threads_bad_thread_id.txt"
	testdataPstackThreadsBadLwp          = "./testdata/linux_pstack_threads_bad_lwp.txt"
)
