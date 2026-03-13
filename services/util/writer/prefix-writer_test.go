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

func TestCleanWriter(t *testing.T) {
	for _, tc := range []struct {
		desc   string
		inputs []string
		output string
	}{
		{
			desc:   "Basic input with prefix",
			inputs: []string{"123-10.0.0.1:9500: some_output_strings\n"},
			output: "some_output_strings\n",
		},
		{
			desc:   "Multiple writes across a single line",
			inputs: []string{"123-10.0.0.1:9500:", " some_output_strings\n"},
			output: "some_output_strings\n",
		},
		{
			desc:   "Two lines, both cleaned",
			inputs: []string{"1:a b\n2:c d\n"},
			output: "b\nd\n",
		},
		{
			desc:   "Line without space remains unchanged",
			inputs: []string{"nospace\n"},
			output: "nospace\n",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer
			w := GetCleanOutputWriter(&buf)
			for _, in := range tc.inputs {
				if _, err := w.Write([]byte(in)); err != nil {
					t.Fatal(err)
				}
			}
			if got, want := buf.String(), tc.output; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestIPOnlyWriter(t *testing.T) {
	for _, tc := range []struct {
		desc   string
		inputs []string
		output string
	}{
		{
			desc:   "IPv4 simple",
			inputs: []string{"123-10.0.0.1:9500: foo\n"},
			output: "10.0.0.1\n",
		},
		{
			desc:   "localhost",
			inputs: []string{"9-localhost:50042: bar\n"},
			output: "localhost\n",
		},
		{
			desc:   "IPv6 bracketed",
			inputs: []string{"77-[2001:db8::1]:50042: baz\n"},
			output: "2001:db8::1\n",
		},
		{
			desc:   "split writes",
			inputs: []string{"1-10.1.1.1:50042:", " abc\n"},
			output: "10.1.1.1\n",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer
			w := GetIPOnlyWriter(&buf)
			for _, in := range tc.inputs {
				if _, err := w.Write([]byte(in)); err != nil {
					t.Fatal(err)
				}
			}
			if got, want := buf.String(), tc.output; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
