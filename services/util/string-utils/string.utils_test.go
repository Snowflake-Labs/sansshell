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

package string_utils

import (
	"testing"
)

func Test_IsAlphanumeric(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult bool
	}{
		{
			name:           "It should return true if the string is alphanumeric",
			input:          "alphanumeric123",
			expectedResult: true,
		},
		{
			name:           "It should return true if the string is alpha",
			input:          "alpha",
			expectedResult: true,
		},
		{
			name:           "It should return true if the string is numeric",
			input:          "1234567890",
			expectedResult: true,
		},
		{
			name:           "It should return false if the string include space",
			input:          "not alphanumeric123",
			expectedResult: false,
		},
		{
			name:           "It should return false if the string include special character",
			input:          "not-alphanumeric123!",
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ACT
			result := IsAlphanumeric(test.input)

			// ASSERT
			if result != test.expectedResult {
				t.Errorf("Expected %t, but got %t", test.expectedResult, result)
			}
		})
	}
}
