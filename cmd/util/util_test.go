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

package util

import (
	"log"
	"testing"
	"time"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func TestValidateAndAddPortAndTimeout(t *testing.T) {
	defer func() { logFatalf = log.Fatalf }() // replace the original func after this test
	tests := []struct {
		name           string
		s              string
		port           int
		dialTimeout    time.Duration
		expectFatal    bool
		expectedResult string
	}{
		{
			name:           "port and timeout exists, shouldn't change anything",
			s:              "localhost:9999;12s",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "localhost:9999;12s",
		},
		{
			name:           "port doesn't exists, should be added",
			s:              "localhost;12s",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "localhost:2345;12s",
		},
		{
			name:           "timeout doesn't exists, should be added",
			s:              "localhost:9999",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "localhost:9999;30s",
		},
		{
			name:           "port and timeout don't exist, both should be added",
			s:              "localhost",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "localhost:2345;30s",
		},
		{
			name:           "invalid timeout without port, should fatal",
			s:              "localhost;3030",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "",
			expectFatal:    true,
		},
		{
			name:           "invalid timeout with port, should fatal",
			s:              "localhost:9999;3030",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "",
			expectFatal:    true,
		},
		{
			name:           "empty string, should fatal",
			s:              "",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "",
			expectFatal:    true,
		},
		{
			name:           "semicolon only, should fatal",
			s:              ";",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "",
			expectFatal:    true,
		},
		{
			name:           "zero timeout shouldn't be added",
			s:              "localhost",
			port:           2345,
			dialTimeout:    0,
			expectedResult: "localhost:2345",
			expectFatal:    false,
		},
	}
	for _, tc := range tests {
		fatalCalled := false
		logFatalf = func(format string, v ...any) {
			fatalCalled = true
		}
		result := ValidateAndAddPortAndTimeout(tc.s, tc.port, tc.dialTimeout)
		testutil.DiffErr(tc.name, fatalCalled, tc.expectFatal, t)

		if !tc.expectFatal {
			testutil.DiffErr(tc.name, result, tc.expectedResult, t)
		}
	}
}

func TestStripTime(t *testing.T) {
	defer func() { logFatalf = log.Fatalf }() // replace the original func after this test
	tests := []struct {
		name           string
		s              string
		expectFatal    bool
		expectedResult string
	}{
		{
			name:           "timeout should be stripped",
			s:              "localhost:9999;20s",
			expectFatal:    false,
			expectedResult: "localhost:9999",
		},
		{
			name:           "no timeout, should return the original string",
			s:              "localhost:9999",
			expectFatal:    false,
			expectedResult: "localhost:9999",
		},
		{
			name:           "no timeout and no port, should return the original string",
			s:              "localhost",
			expectFatal:    false,
			expectedResult: "localhost",
		},
		{
			name:           "multiple timeout should fatal",
			s:              "localhost:9999;20s;30s",
			expectFatal:    true,
			expectedResult: "",
		},
	}
	for _, tc := range tests {
		fatalCalled := false
		logFatalf = func(format string, v ...any) {
			fatalCalled = true
		}
		result := StripTimeout(tc.s)
		testutil.DiffErr(tc.name, fatalCalled, tc.expectFatal, t)
		if !tc.expectFatal {
			testutil.DiffErr(tc.name, result, tc.expectedResult, t)
		}
	}
}
