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

// GetIPOnlyWriter returns a writer that, for each line written to it,
// extracts the remote host IP (or host) from a header of the form
// "<random>-<host-or-ip>:<port>: ..." and writes only the IP/host followed by a newline.
//
// Examples:
//
//	"123-10.0.0.1:9500: some_output_strings\n" -> "10.0.0.1\n"
//	"77-[2001:db8::1]:50042: foo\n" -> "2001:db8::1\n"
//	"9-localhost:50042: bar\n" -> "localhost\n"
//
// The writer is newline-aware and will correctly handle content split across multiple writes.
func GetIPOnlyWriter(dest io.Writer) WrappedWriter {
	return &ipOnlyWriter{dest: dest}
}

type ipOnlyWriter struct {
	dest io.Writer
	buf  []byte
}

func (w *ipOnlyWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	total := len(p)
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx == -1 {
			break
		}
		line := w.buf[:idx+1]
		// Extract section before first space (header)
		headerEnd := bytes.IndexByte(line, ' ')
		header := line
		if headerEnd >= 0 {
			header = line[:headerEnd]
		}
		// From header, find the first '-' and the next ':' after it
		ipStart := bytes.IndexByte(header, '-')
		var host []byte
		if ipStart >= 0 && ipStart+1 < len(header) {
			rest := header[ipStart+1:]
			// If IPv6 is bracketed like [2001:db8::1], handle brackets
			if len(rest) > 0 && rest[0] == '[' {
				// Find closing bracket
				rb := bytes.IndexByte(rest, ']')
				if rb >= 0 {
					host = rest[1:rb]
				}
			}
			if host == nil {
				// Take up to the first ':' which precedes the port
				colon := bytes.IndexByte(rest, ':')
				if colon >= 0 {
					host = rest[:colon]
				} else {
					host = rest
				}
			}
		}
		host = bytes.TrimSpace(host)
		if host == nil {
			host = []byte{}
		}
		// Write host plus newline
		if _, err := w.dest.Write(append(host, '\n')); err != nil {
			return 0, err
		}
		w.buf = w.buf[idx+1:]
	}
	return total, nil
}

func (w *ipOnlyWriter) GetOriginal() io.Writer {
	return w.dest
}
