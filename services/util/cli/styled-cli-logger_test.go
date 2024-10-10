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
	"bytes"
	"strings"
	"testing"
)

func escapeAnsiCodes(s string) string {
	ansi := []struct {
		s string
		r string
	}{
		{
			s: "\033",
			r: "\\033",
		},
		{
			s: "\u001B",
			r: "\\u001B",
		},
	}

	for _, code := range ansi {
		s = strings.ReplaceAll(s, code.s, code.r)
	}
	return s
}

func TestColorize(t *testing.T) {
	t.Run("It should create stringify-able what returns original string", func(t *testing.T) {
		// ARRANGE
		text := "Some text"
		color := RedText

		// ACT
		colorized := Colorize(color, text)

		// ASSERT
		s := colorized.String()

		// it should be
		if s != text {
			t.Errorf("Got %s, but expected %s", s, text)
		}
	})
}

func TestColorizef(t *testing.T) {
	t.Run("It should create stringify-able what returns formatted string", func(t *testing.T) {
		// ARRANGE
		text := "Some text: %s"
		substring := "some substring"
		color := RedText
		expectedResult := "Some text: some substring"

		// ACT
		colorized := Colorizef(color, text, substring)

		// ASSERT
		s := colorized.String()

		// it should be
		if s != expectedResult {
			t.Errorf("Got %s, but expected %s", s, text)
		}
	})
}

func TestCRed(t *testing.T) {
	t.Run("It should convert string to red styled text", func(t *testing.T) {
		// ARRANGE
		text := "Some text"

		// ACT
		colorized := CRed(text)

		// ASSERT
		styled, ok := colorized.(*styledText)
		if ok == false {
			t.Errorf("Expected to get styledText, but got %T", colorized)
		}

		if styled.text != text {
			t.Errorf("Got %s, but expected %s", styled.text, text)
		}

		if styled.colorCode != RedText {
			t.Errorf("Got %s, but expected %s", styled.colorCode, RedText)
		}
	})
}

func TestCGreen(t *testing.T) {
	t.Run("It should convert string to green styled text", func(t *testing.T) {
		// ARRANGE
		text := "Some text"

		// ACT
		colorized := CGreen(text)

		// ASSERT
		styled, ok := colorized.(*styledText)
		if ok == false {
			t.Errorf("Expected to get styledText, but got %T", colorized)
		}

		if styled.text != text {
			t.Errorf("Got %s, but expected %s", styled.text, text)
		}

		if styled.colorCode != GreenText {
			t.Errorf("Got %s, but expected %s", styled.colorCode, GreenText)
		}
	})
}

func Test_styledCliLogger_Infof(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		arg1           any
		arg2           any
		enableStyling  bool
		expectedOutput string
	}{
		{
			name:           "It should print formatted string to out writer",
			format:         "Some format %d %s",
			arg1:           80,
			arg2:           "Some string",
			enableStyling:  false,
			expectedOutput: "Some format 80 Some string",
		},
		{
			name:           "It should print formatted string WITHOUT styling to out writer, in case styling disabled",
			format:         "Some format %d %s",
			arg1:           80,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  false,
			expectedOutput: "Some format 80 Some string",
		},
		{
			name:           "It should print formatted string WITH styling to out writer, in case styling enabled",
			format:         "Some format %d %s",
			arg1:           80,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  true,
			expectedOutput: "Some format 80 \033[31mSome string\033[0m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var out bytes.Buffer
			var err bytes.Buffer
			logger := NewStyledCliLogger(&out, &err, &CliLoggerOptions{
				ApplyStylingForOut: test.enableStyling,
				ApplyStylingForErr: false,
			})

			// ACT
			logger.Infof(test.format, test.arg1, test.arg2)

			// ASSERT
			result := out.String()

			if result != test.expectedOutput {
				escapedResult := escapeAnsiCodes(result)
				escapedExpectedOutput := escapeAnsiCodes(test.expectedOutput)
				t.Errorf("Expected \"%s\", but provided \"%s\"", escapedExpectedOutput, escapedResult)
			}
		})
	}
}

func Test_styledCliLogger_Info(t *testing.T) {
	tests := []struct {
		name           string
		arg1           any
		arg2           any
		enableStyling  bool
		expectedOutput string
	}{
		{
			name:           "It should print string to out writer",
			arg1:           80,
			arg2:           "Some string",
			enableStyling:  false,
			expectedOutput: "80Some string",
		},
		{
			name:           "It should print string WITHOUT styling to out writer, in case styling disabled",
			arg1:           80,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  false,
			expectedOutput: "80Some string",
		},
		{
			name:           "It should print string WITH styling to out writer, in case styling enabled",
			arg1:           80,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  true,
			expectedOutput: "80\033[31mSome string\033[0m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var out bytes.Buffer
			var err bytes.Buffer
			logger := NewStyledCliLogger(&out, &err, &CliLoggerOptions{
				ApplyStylingForOut: test.enableStyling,
				ApplyStylingForErr: false,
			})

			// ACT
			logger.Info(test.arg1, test.arg2)

			// ASSERT
			result := out.String()

			if result != test.expectedOutput {
				escapedResult := escapeAnsiCodes(result)
				escapedExpectedOutput := escapeAnsiCodes(test.expectedOutput)
				t.Errorf("Expected \"%s\", but provided \"%s\"", escapedExpectedOutput, escapedResult)
			}
		})
	}
}

func Test_styledCliLogger_Errorf(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		arg1           any
		arg2           any
		enableStyling  bool
		expectedOutput string
	}{
		{
			name:           "It should print formatted string to err writer",
			format:         "Some format %d %s",
			arg1:           80,
			arg2:           "Some string",
			enableStyling:  false,
			expectedOutput: "Some format 80 Some string",
		},
		{
			name:           "It should print formatted string WITHOUT styling to err writer, in case styling disabled",
			format:         "Some format %d %s",
			arg1:           80,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  false,
			expectedOutput: "Some format 80 Some string",
		},
		{
			name:           "It should print formatted string WITH styling to err writer, in case styling enabled",
			format:         "Some format %d %s",
			arg1:           80,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  true,
			expectedOutput: "Some format 80 \033[31mSome string\033[0m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var out bytes.Buffer
			var err bytes.Buffer
			logger := NewStyledCliLogger(&out, &err, &CliLoggerOptions{
				ApplyStylingForOut: false,
				ApplyStylingForErr: test.enableStyling,
			})

			// ACT
			logger.Errorf(test.format, test.arg1, test.arg2)

			// ASSERT
			result := err.String()

			if result != test.expectedOutput {
				escapedResult := escapeAnsiCodes(result)
				escapedExpectedOutput := escapeAnsiCodes(test.expectedOutput)
				t.Errorf("Expected \"%s\", but provided \"%s\"", escapedExpectedOutput, escapedResult)
			}
		})
	}
}

func Test_styledCliLogger_Error(t *testing.T) {
	tests := []struct {
		name           string
		arg1           any
		arg2           any
		enableStyling  bool
		expectedOutput string
	}{
		{
			name:           "It should print string to err writer",
			arg1:           80,
			arg2:           "Some string",
			enableStyling:  false,
			expectedOutput: "80Some string",
		},
		{
			name:           "It should print string WITHOUT styling to err writer, in case styling disabled",
			arg1:           80,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  false,
			expectedOutput: "80Some string",
		},
		{
			name:           "It should print string WITH styling to err writer, in case styling enabled",
			arg1:           80,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  true,
			expectedOutput: "80\033[31mSome string\033[0m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var out bytes.Buffer
			var err bytes.Buffer
			logger := NewStyledCliLogger(&out, &err, &CliLoggerOptions{
				ApplyStylingForOut: test.enableStyling,
				ApplyStylingForErr: false,
			})

			// ACT
			logger.Error(test.arg1, test.arg2)

			// ASSERT
			result := err.String()

			if result != test.expectedOutput {
				escapedResult := escapeAnsiCodes(result)
				escapedExpectedOutput := escapeAnsiCodes(test.expectedOutput)
				t.Errorf("Expected \"%s\", but provided \"%s\"", escapedExpectedOutput, escapedResult)
			}
		})
	}
}

func Test_styledCliLogger_Errorc(t *testing.T) {
	tests := []struct {
		name           string
		colorCode      ColorCode
		arg1           any
		arg2           any
		enableStyling  bool
		expectedOutput string
	}{
		{
			name:           "It should print string to err writer WITHOUT styling",
			colorCode:      RedText,
			arg1:           80,
			arg2:           "Some string",
			enableStyling:  false,
			expectedOutput: "80Some string",
		},
		{
			name:           "It should print string WITH styling to err writer, in case styling enabled",
			arg1:           80,
			colorCode:      RedText,
			arg2:           Colorize(RedText, "Some string"),
			enableStyling:  false,
			expectedOutput: "80Some string",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var out bytes.Buffer
			var err bytes.Buffer
			logger := NewStyledCliLogger(&out, &err, &CliLoggerOptions{
				ApplyStylingForOut: test.enableStyling,
				ApplyStylingForErr: false,
			})

			// ACT
			logger.Errorc(test.colorCode, test.arg1, test.arg2)

			// ASSERT
			result := err.String()

			if result != test.expectedOutput {
				escapedResult := escapeAnsiCodes(result)
				escapedExpectedOutput := escapeAnsiCodes(test.expectedOutput)
				t.Errorf("Expected \"%s\", but provided \"%s\"", escapedExpectedOutput, escapedResult)
			}
		})
	}
}

func Test_styledCliLogger_Errorcf(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		colorCode      ColorCode
		arg1           any
		arg2           any
		enableStyling  bool
		expectedOutput string
	}{
		{
			name:           "It should print string to err writer WITHOUT styling",
			format:         "%d %s",
			colorCode:      RedText,
			arg1:           80,
			arg2:           "Some string",
			enableStyling:  false,
			expectedOutput: "80 Some string",
		},
		{
			name:           "It should print string WITH styling to err writer, in case styling enabled",
			format:         "%d %s",
			colorCode:      RedText,
			arg1:           80,
			arg2:           "Some string",
			enableStyling:  true,
			expectedOutput: "\033[31m80 Some string\033[0m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var out bytes.Buffer
			var err bytes.Buffer
			logger := NewStyledCliLogger(&out, &err, &CliLoggerOptions{
				ApplyStylingForErr: test.enableStyling,
			})

			// ACT
			logger.Errorfc(test.colorCode, test.format, test.arg1, test.arg2)

			// ASSERT
			result := err.String()

			if result != test.expectedOutput {
				escapedResult := escapeAnsiCodes(result)
				escapedExpectedOutput := escapeAnsiCodes(test.expectedOutput)
				t.Errorf("Expected \"%s\", but provided \"%s\"", escapedExpectedOutput, escapedResult)
			}
		})
	}
}
