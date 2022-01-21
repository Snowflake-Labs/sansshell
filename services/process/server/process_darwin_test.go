//go:build darwin
// +build darwin

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
	testdataPsTextProto = "./testdata/darwin_testdata.ps.textproto"
	testdataPs          = "./testdata/darwin.ps"
	badFilesPs          = []string{
		"./testdata/darwin_bad0.ps", // empty file
		"./testdata/darwin_bad1.ps", // bad pid entry
		"./testdata/darwin_bad2.ps", // No entries (multiple blank lines)
		"./testdata/darwin_bad3.ps", // Bad cpu value
		"./testdata/darwin_bad4.ps", // Bad mem value
	}

	// Technically Darwin has no support but for precanned things we can use the linux test data.
	// Tests using the native pstack will still be skipped.
	testdataPstackNoThreads              = "./testdata/linux_pstack_no_threads.txt"
	testdataPstackNoThreadsTextProto     = "./testdata/linux_pstack_no_threads.textproto"
	testdataPstackThreads                = "./testdata/linux_pstack_threads.txt"
	testdataPstackThreadsTextProto       = "./testdata/linux_pstack_threads.textproto"
	testdataPstackThreadsBadThread       = "./testdata/linux_pstack_threads_bad_thread.txt"
	testdataPstackThreadsBadThreadNumber = "./testdata/linux_pstack_threads_bad_thread_number.txt"
	testdataPstackThreadsBadThreadId     = "./testdata/linux_pstack_threads_bad_thread_id.txt"
	testdataPstackThreadsBadLwp          = "./testdata/linux_pstack_threads_bad_lwp.txt"
)
