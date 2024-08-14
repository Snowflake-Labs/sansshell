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

package validator

import (
	"testing"
)

func TestParsePortFromInt(t *testing.T) {
	tests := []struct {
		input    int
		expected uint32
		err      bool
		name     string
	}{
		{
			name:     "It should successfully parse port from int",
			input:    80,
			expected: 80,
			err:      false,
		},
		{
			name:     "It should successfully parse maximum allowed port value",
			input:    65535,
			expected: 65535,
			err:      false,
		},
		{
			name:     "It should fail if zero provided",
			input:    0,
			expected: 0,
			err:      true,
		},
		{
			name:     "It should fail if provided value is greater than maximum allowed port value",
			input:    65536,
			expected: 0,
			err:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ACT
			result, err := ParsePortFromInt(test.input)

			// ASSERT
			if (err != nil) != test.err {
				t.Errorf("ParsePortFromInt(%d) error = %v, expected error = %v", test.input, err, test.err)
			}
			if result != test.expected {
				t.Errorf("ParsePortFromInt(%d) = %d, expected %d", test.input, result, test.expected)
			}
		})
	}
}

func TestParsePortFromUint32(t *testing.T) {
	tests := []struct {
		input    uint32
		expected uint32
		err      bool
		name     string
	}{
		{
			name:     "It should successfully parse port from int",
			input:    80,
			expected: 80,
			err:      false,
		},
		{
			name:     "It should successfully parse maximum allowed port value",
			input:    65535,
			expected: 65535,
			err:      false,
		},
		{
			name:     "It should fail if zero provided",
			input:    0,
			expected: 0,
			err:      true,
		},
		{
			name:     "It should fail if provided value is greater than maximum allowed port value",
			input:    65536,
			expected: 0,
			err:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ACT
			result, err := ParsePortFromUint32(test.input)

			// ASSERT
			if (err != nil) != test.err {
				t.Errorf("ParsePortFromInt(%d) error = %v, expected error = %v", test.input, err, test.err)
			}
			if result != test.expected {
				t.Errorf("ParsePortFromInt(%d) = %d, expected %d", test.input, result, test.expected)
			}
		})
	}
}

func TestParseHostAndPort(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedHost string
		expectedPort uint32
		err          bool
	}{
		{
			name:         "it should parse valid host name and port",
			input:        "localhost:80",
			expectedHost: "localhost",
			expectedPort: 80,
			err:          false,
		},
		{
			name:         "it should parse valid ip and port",
			input:        "127.0.0.1:80",
			expectedHost: "127.0.0.1",
			expectedPort: 80,
			err:          false,
		},
		{
			name:         "it should fail if only named host was provided",
			input:        "localhost",
			expectedHost: "",
			expectedPort: 0,
			err:          true,
		},
		{
			name:         "it should fail if only ip was provided",
			input:        "127.0.0.1",
			expectedHost: "",
			expectedPort: 0,
			err:          true,
		},
		{
			name:         "it should fail if only port was provided",
			input:        ":22",
			expectedHost: "",
			expectedPort: 0,
			err:          true,
		},
		{
			name:         "it should fail if NOT numeric port was provided",
			input:        "localhost:abc",
			expectedHost: "",
			expectedPort: 0,
			err:          true,
		},
		{
			name:         "it should fail if port value is too big was provided",
			input:        "localhost:65536",
			expectedHost: "",
			expectedPort: 0,
			err:          true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host, port, err := ParseHostAndPort(test.input)
			if (err != nil) != test.err {
				t.Errorf("ParseHostAndPort(%s) error = %v, expected error = %v", test.input, err, test.err)
			}
			if host != test.expectedHost {
				t.Errorf("ParseHostAndPort(%s) host = %s, expected %s", test.input, host, test.expectedHost)
			}
			if port != test.expectedPort {
				t.Errorf("ParseHostAndPort(%s) port = %d, expected %d", test.input, port, test.expectedPort)
			}
		})
	}
}
