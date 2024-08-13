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

package cli

import (
	"fmt"
	"io"
	"os"
)

type ColorCode = string

var restoreFormatingCode ColorCode = "\033[0m"
var RedText ColorCode = "\033[31m"
var GreenText ColorCode = "\033[32m"
var YellowText ColorCode = "\033[33m"

type styledCliLogger struct {
}

func (l *styledCliLogger) coloredSprint(color string, a ...any) string {
	line := fmt.Sprint(a...)
	return fmt.Sprint(color, line, restoreFormatingCode)
}

func (l *styledCliLogger) applyStyleByStream(stream io.Writer, a ...any) []any {
	if IsStreamToTerminal(stream) {
		newA := make([]any, len(a))
		for i, v := range a {
			if styledText, ok := v.(*styledText); ok {
				v = l.coloredSprint(styledText.colorCode, styledText.text)
			}

			newA[i] = v
		}
		a = newA
	}

	return a
}

func (l *styledCliLogger) fprintf(stream io.Writer, format string, a ...any) (n int, err error) {
	a = l.applyStyleByStream(stream, a...)
	return fmt.Fprintf(stream, format, a...)
}

func (l *styledCliLogger) fprint(stream io.Writer, a ...any) (n int, err error) {
	a = l.applyStyleByStream(stream, a...)
	return fmt.Fprint(stream, a...)
}

func (l *styledCliLogger) Infof(format string, a ...any) {
	l.fprintf(os.Stdout, format, a...)
}

func (l *styledCliLogger) Info(a ...any) {
	l.fprint(os.Stdout, a...)
}

func (l *styledCliLogger) Error(a ...any) {
	l.fprint(os.Stderr, a...)
}

func (l *styledCliLogger) Errorf(format string, a ...any) {
	l.fprintf(os.Stderr, format, a...)
}

// Cerrorf prints error message with color, styling inside format is not supported
func (l *styledCliLogger) Errorfc(color ColorCode, format string, a ...any) {
	isPrintToTerminal := IsStreamToTerminal(os.Stderr)

	if isPrintToTerminal {
		fmt.Fprint(os.Stderr, color)
	}

	fmt.Fprintf(os.Stderr, format, a...)

	if isPrintToTerminal {
		fmt.Fprint(os.Stderr, restoreFormatingCode)
	}
}

// Cerror prints error message with color, styling inside format is not supported
func (l *styledCliLogger) Errorc(color ColorCode, a ...any) {
	isPrintToTerminal := IsStreamToTerminal(os.Stderr)

	if isPrintToTerminal {
		fmt.Fprint(os.Stderr, color)
	}

	fmt.Fprint(os.Stderr, a...)

	if isPrintToTerminal {
		fmt.Fprint(os.Stderr, restoreFormatingCode)
	}

}

type StyledCliLogger interface {
	Infof(format string, a ...any)
	Info(a ...any)
	Errorf(format string, a ...any)
	Error(a ...any)
	Errorc(color ColorCode, a ...any)
	Errorfc(color ColorCode, format string, a ...any)
}

func NewStyledCliLogger() StyledCliLogger {
	return &styledCliLogger{}
}

type styledText struct {
	text      string
	colorCode string
}

func (s *styledText) String() string {
	return s.text
}

type StyledText interface {
	String() string
}

func Colorize(color ColorCode, text string) StyledText {
	return &styledText{
		text:      text,
		colorCode: color,
	}
}

func Colorizef(color ColorCode, format string, a ...any) StyledText {
	return &styledText{
		text:      fmt.Sprintf(format, a...),
		colorCode: color,
	}
}
