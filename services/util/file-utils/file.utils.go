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

package file_utils

import (
	"fmt"
	"os"
	"syscall"
)

// OpenForOverwrite opens a file for writing, truncating the file if it already exists.
func OpenForOverwrite(filePath string) (*os.File, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %s", err.Error())
	}
	mode := fileInfo.Mode()

	// Open the file in read-write mode
	f, err := os.OpenFile(filePath, os.O_RDWR, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %s", err)
	}

	return f, nil
}

// ExclusiveLockFile applies an exclusive lock to a file.
// Returns a function to unlock the file.
func ExclusiveLockFile(f *os.File) (func() error, error) {
	// Apply an exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, fmt.Errorf("failed to lock file: %s", err)
	}

	// Return a function to unlock the file
	return func() error {
		return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	}, nil
}
