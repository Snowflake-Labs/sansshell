/*
Copyright (c) 2019 Snowflake Inc. All rights reserved.

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

package array_utils

import (
	"testing"
)

func Test_FindIndexBy(t *testing.T) {
	tests := []struct {
		name           string
		arr            []int
		predicate      func(val int) bool
		expectedResult int
	}{
		{
			name:           "It should return index of value in array",
			arr:            []int{1, 2, 3, 4, 5},
			predicate:      func(val int) bool { return val == 3 },
			expectedResult: 2,
		},
		{
			name:           "It should return -1 if value is not in array",
			arr:            []int{1, 2, 3, 4, 5},
			predicate:      func(val int) bool { return val == 6 },
			expectedResult: -1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ACT
			result := FindIndexBy(test.arr, test.predicate)

			// ASSERT
			if result != test.expectedResult {
				t.Errorf("Expected %d, but got %d", test.expectedResult, result)
			}
		})
	}
}
