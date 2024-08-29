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

package domain

import (
	pb "github.com/Snowflake-Labs/sansshell/services/packages"
	"testing"
)

func TestToNevraStr(t *testing.T) {
	tests := []struct {
		name     string
		input    *pb.PackageInfo
		expected string
	}{
		{
			name:     "It should convert package info to NEVRA string",
			input:    &pb.PackageInfo{Name: "test", Epoch: 1, Version: "1.0.0", Release: "1", Architecture: "x86_64"},
			expected: "test-1:1.0.0-1.x86_64",
		},
		{
			name:     "It should convert package info to NEVRA string, in case of epoch is 0",
			input:    &pb.PackageInfo{Name: "test", Epoch: 0, Version: "1.0.0", Release: "1", Architecture: "x86_64"},
			expected: "test-0:1.0.0-1.x86_64",
		},
		{
			name:     "It should convert package info to NEVRA string, in case of version is not in XX.XX.XX format",
			input:    &pb.PackageInfo{Name: "test", Epoch: 0, Version: "1.10.22_rc-2", Release: "1", Architecture: "x86_64"},
			expected: "test-0:1.10.22_rc-2-1.x86_64",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ACT
			result := ToNevraStr(test.input)

			// ASSERT
			if result != test.expected {
				t.Errorf("Expected %s, but provided %s", test.expected, result)
			}
		})
	}
}
