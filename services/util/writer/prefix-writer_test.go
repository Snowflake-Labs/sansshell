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

package writer

import (
	"bytes"
	"testing"
)

func TestPrefixWriter(t *testing.T) {

	for _, tc := range []struct {
		desc   string
		inputs []string
		output string
	}{
		{
			desc:   "Basic input",
			inputs: []string{"Hello world\n"},
			output: "prefix: Hello world\n",
		},
		{
			desc:   "Multiple writes for single line",
			inputs: []string{"Hello ", "world\n"},
			output: "prefix: Hello world\n",
		},
		{
			desc:   "Two lines, single write",
			inputs: []string{"Hello\nworld\n"},
			output: "prefix: Hello\nprefix: world\n",
		},
		{
			desc:   "Two lines, multiple writes",
			inputs: []string{"Hello\n", "world\n"},
			output: "prefix: Hello\nprefix: world\n",
		},
		{
			desc:   "Three lines, single write",
			inputs: []string{"Hello\nworld\n\n"},
			output: "prefix: Hello\nprefix: world\nprefix: \n",
		},
		{
			desc:   "Empty input",
			inputs: []string{""},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer
			writer := &prefixWriter{
				start:  true,
				dest:   &buf,
				prefix: []byte("prefix: "),
			}

			for _, i := range tc.inputs {
				if _, err := writer.Write([]byte(i)); err != nil {
					t.Error(err)
				}
			}
			got := buf.String()
			if got != tc.output {
				t.Errorf("got %q, want %q", got, tc.output)
			}

		})
	}

	t.Run("GetOriginal should return original writer", func(t *testing.T) {
		// ARRANGE
		var buf bytes.Buffer
		writer := &prefixWriter{
			start:  true,
			dest:   &buf,
			prefix: []byte("prefix: "),
		}

		// ACT
		originalWriter := writer.GetOriginal()

		// ASSERT
		if originalWriter != &buf {
			t.Errorf("got %q, want %q", originalWriter, &buf)
		}
	})
}
