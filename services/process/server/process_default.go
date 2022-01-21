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
	"flag"
	"fmt"
	"io"
	"runtime"
)

var (
	psBin     = flag.String("ps-bin", "", "Path to the ps binary")
	pstackBin = flag.String("pstack-bin", "", "Path to the pstack binary")
	gcoreBin  = flag.String("gcore-bin", "", "Path to the gcore binary")

	psOptions = func() ([]string, error) {
		return nil, fmt.Errorf("No support for OS %s", runtime.GOOS)
	}
)

func parser(r io.Reader) (map[int64]*ProcessEntry, error) {
	return nil, fmt.Errorf("No support for OS %s", runtime.GOOS)
}
