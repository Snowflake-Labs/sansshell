//go:build linux
// +build linux

/*
Copyright (c) 2023 Snowflake Inc. All rights reserved.

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
package server

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func TestReadJournalLine(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("hello\nworld\n"))

	line, trunc, err := readJournalLine(r, 1024)
	if string(line) != "hello" || trunc || err != nil {
		t.Fatalf("first line: got (%q,%v,%v), want (hello,false,nil)", line, trunc, err)
	}

	line, trunc, err = readJournalLine(r, 1024)
	if string(line) != "world" || trunc || err != nil {
		t.Fatalf("second line: got (%q,%v,%v), want (world,false,nil)", line, trunc, err)
	}

	line, trunc, err = readJournalLine(r, 1024)
	if len(line) != 0 || trunc || err != io.EOF {
		t.Fatalf("eof: got (%q,%v,%v), want (\"\",false,EOF)", line, trunc, err)
	}
}

func TestReadJournalLineTruncates(t *testing.T) {
	// A single line larger than both the cap and bufio's internal buffer,
	// followed by a short line. The long line must be reported truncated (kept to
	// the cap) and the following line must still be readable so streaming can
	// continue rather than silently stopping.
	long := strings.Repeat("A", 10000)
	r := bufio.NewReader(strings.NewReader(long + "\nshort\n"))

	line, trunc, err := readJournalLine(r, 100)
	if !trunc {
		t.Fatalf("expected truncated=true for oversized line")
	}
	if len(line) != 100 {
		t.Fatalf("expected 100 bytes retained, got %d", len(line))
	}
	if err != nil {
		t.Fatalf("unexpected error on truncated line: %v", err)
	}

	line, trunc, err = readJournalLine(r, 100)
	if string(line) != "short" || trunc || err != nil {
		t.Fatalf("line after truncation: got (%q,%v,%v), want (short,false,nil)", line, trunc, err)
	}
}

func TestReadJournalLineNoTrailingNewline(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("tail"))

	line, trunc, err := readJournalLine(r, 1024)
	if string(line) != "tail" || trunc {
		t.Fatalf("got (%q,%v), want (tail,false)", line, trunc)
	}
	if err != io.EOF {
		t.Fatalf("want io.EOF, got %v", err)
	}
}
