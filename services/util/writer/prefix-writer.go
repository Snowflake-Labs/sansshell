/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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

type WrappedWriter interface {
	GetOriginal() io.Writer
	Write(p []byte) (n int, err error)
}

func GetPrefixedWriter(prefix []byte, start bool, dest io.Writer) WrappedWriter {
	return &prefixWriter{
		prefix: prefix,
		start:  start,
		dest:   dest,
	}
}

type prefixWriter struct {
	prefix []byte
	start  bool
	dest   io.Writer
}

func (p *prefixWriter) Write(b []byte) (n int, err error) {
	// Exit early if we're not writing any bytes
	if len(b) == 0 {
		return 0, nil
	}

	// Keep track of the size of the incoming buf as we'll print more
	// but clients want to know we wrote what they asked for.
	tot := len(b)

	// If we just started emit a prefix
	if p.start {
		n, err = p.dest.Write(p.prefix)
		if err != nil {
			return n, err
		}
		p.start = false
	}

	// Find any newlines and augment them with the prefix appended onto them.

	// If the last byte is a newline we don't want to add a prefix yet as this may be the end of output.
	// Instead we'll remark start so the next one prints and remove the newline for now.
	if n := bytes.LastIndex(b, []byte{'\n'}); n == len(b)-1 {
		p.start = true
		b = b[:len(b)-1]
	}
	b = bytes.ReplaceAll(b, []byte{'\n'}, append(append([]byte{}, byte('\n')), p.prefix...))
	// If start got set above we need to add back the newline we dropped so output looks correct.
	// Thankfully b is now a new slice as Write() isn't supposed to directly modify the incoming one.
	if p.start {
		b = append(b, '\n')
	}
	n, err = p.dest.Write(b)
	if err != nil {
		return n, err
	}
	return tot, nil
}

func (p *prefixWriter) GetOriginal() io.Writer {
	return p.dest
}
