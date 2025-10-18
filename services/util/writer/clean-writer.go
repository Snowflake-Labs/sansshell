/* Copyright (c) 2025 Snowflake Inc. All rights reserved.

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
	"io"
)

// GetCleanOutputWriter returns a writer that, for each line written to it,
// removes everything from the beginning of the line up to and including the
// first space character, then writes the remainder to the underlying writer.
//
// This is useful for transforming lines like:
//
//	"123-10.0.0.1:9500: some_output_strings\n"
//
// into:
//
//	"some_output_strings\n"
//
// The writer is newline-aware and will correctly handle content split across
// multiple Write() calls.
func GetCleanOutputWriter(dest io.Writer) WrappedWriter {
	return &cleanWriter{
		dest: dest,
	}
}

type cleanWriter struct {
	dest io.Writer
	buf  []byte
}

func (c *cleanWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	// Keep track of input length to report as written count regardless of
	// transformation performed.
	total := len(p)

	// Accumulate into buffer and flush full lines.
	c.buf = append(c.buf, p...)
	for {
		idx := bytes.IndexByte(c.buf, '\n')
		if idx == -1 {
			break
		}
		line := c.buf[:idx+1] // include the newline

		// Find the first space and strip everything up to and including it.
		out := line
		if sp := bytes.IndexByte(line, ' '); sp >= 0 {
			out = line[sp+1:]
		}
		if _, err := c.dest.Write(out); err != nil {
			return 0, err
		}
		// Advance buffer
		c.buf = c.buf[idx+1:]
	}
	return total, nil
}

func (c *cleanWriter) GetOriginal() io.Writer {
	return c.dest
}
