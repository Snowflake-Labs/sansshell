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
)

type ColorCode = string

var restoreFormatingCode ColorCode = "\033[0m"
var RedText ColorCode = "\033[31m"
var GreenText ColorCode = "\033[32m"
var YellowText ColorCode = "\033[33m"

type styledCliLogger struct {
	out               io.Writer
	outStylingEnabled bool
	err               io.Writer
	errStylingEnabled bool
}

func (l *styledCliLogger) coloredSprint(color string, a ...any) string {
	line := fmt.Sprint(a...)
	return fmt.Sprint(color, line, restoreFormatingCode)
}

func (l *styledCliLogger) toPrimitives(applyStyling bool, a ...any) []any {
	newA := make([]any, len(a))
	for i, v := range a {
		if styledText, ok := v.(*styledText); ok {
			if applyStyling {
				v = l.coloredSprint(styledText.colorCode, styledText.text)
			} else {
				v = styledText.text
			}
		}

		newA[i] = v
	}
	return newA
}

func (l *styledCliLogger) fprintf(stream io.Writer, applyStyling bool, format string, a ...any) (n int, err error) {
	a = l.toPrimitives(applyStyling, a...)
	return fmt.Fprintf(stream, format, a...)
}

func (l *styledCliLogger) fprint(stream io.Writer, applyStyling bool, a ...any) (n int, err error) {
	a = l.toPrimitives(applyStyling, a...)
	return fmt.Fprint(stream, a...)
}

func (l *styledCliLogger) Infof(format string, a ...any) {
	l.fprintf(l.out, l.outStylingEnabled, format, a...)
}

func (l *styledCliLogger) Info(a ...any) {
	l.fprint(l.out, l.outStylingEnabled, a...)
}

func (l *styledCliLogger) Error(a ...any) {
	l.fprint(l.err, l.outStylingEnabled, a...)
}

func (l *styledCliLogger) Errorf(format string, a ...any) {
	l.fprintf(l.err, l.errStylingEnabled, format, a...)
}

// Errorfc prints error message with color, styling inside format is not supported
func (l *styledCliLogger) Errorfc(color ColorCode, format string, a ...any) {
	if l.errStylingEnabled {
		l.Error(color)
	}

	l.Errorf(format, a...)

	if l.errStylingEnabled {
		l.Error(restoreFormatingCode)
	}
}

// Errorc prints error message with color, styling inside format is not supported
func (l *styledCliLogger) Errorc(color ColorCode, a ...any) {
	if l.errStylingEnabled {
		l.Error(color)
	}

	l.Error(a...)

	if l.errStylingEnabled {
		l.Error(restoreFormatingCode)
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

type CliLoggerOptions struct {
	ApplyStylingForOut bool
	ApplyStylingForErr bool
}

func NewStyledCliLogger(out io.Writer, err io.Writer, options *CliLoggerOptions) StyledCliLogger {
	return &styledCliLogger{
		out:               out,
		outStylingEnabled: options.ApplyStylingForOut,
		err:               err,
		errStylingEnabled: options.ApplyStylingForErr,
	}
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
