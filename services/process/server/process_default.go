//go:build !(linux || darwin)
// +build !linux,!darwin

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

import (
	"fmt"
	"io"
	"runtime"

	pb "github.com/Snowflake-Labs/sansshell/services/process"
)

var (
	// PsBin is the location of the ps binary. On non linux/OS/X this isn't supported.
	PsBin = ""

	// PstackBin is the location of the pstack binary. On non linux/OS/X this isn't supported.
	PstackBin = ""

	// GcoreBin is the location of the gcore binary. On non linux/OS/X this isn't supported.
	GcoreBin = ""

	psOptions = func() []string {
		return nil
	}
)

func parser(r io.Reader) (map[int64]*pb.ProcessEntry, error) {
	return nil, fmt.Errorf("No support for OS %s", runtime.GOOS)
}
