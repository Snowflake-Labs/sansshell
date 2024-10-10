/*
Copyright (c) 2024 Snowflake Inc. All rights reserved.

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
package yml_utils

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func getLinesDiff(expected string, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")
	var diffLines []string
	for i := 0; i < min(len(expectedLines), len(actualLines)); i++ {
		line := expectedLines[i]
		actualLine := actualLines[i]

		if line == actualLine {
			diffLines = append(diffLines, line)
		} else {
			diffLines = append(diffLines, fmt.Sprintf("-%s", line))
			diffLines = append(diffLines, fmt.Sprintf("+%s", actualLine))
		}
	}

	if len(expectedLines) > len(actualLines) {
		diffLines = append(diffLines, expectedLines[len(actualLines):]...)
	}

	if len(actualLines) > len(expectedLines) {
		for _, line := range actualLines[len(expectedLines):] {
			diffLines = append(diffLines, fmt.Sprintf("+%s", line))
		}
	}

	return strings.Join(diffLines, "\n")
}

func Test_Yaml3_GetScalarValueFrom(t *testing.T) {
	sourceYaml := `
root:
  # top comment
  simple_key: simple_key_value # simple key comment
  simple_sequence:
    # simple sequence comment
    - simple_sequence_value_1
    - simple_sequence_value_2
    - simple_sequence_value_3
  nested_sequence: # simple sequence comment
    - nested_sequence_value_1:
        nested_sequence_key_1: nested_sequence_value_1_key_1
        nested_sequence_key_2: nested_sequence_value_1_key_2
    - nested_sequence_value_2:
        nested_sequence_key_1: nested_sequence_value_2_key_1
        nested_sequence_key_2: nested_sequence_value_2_key_2
    - nested_sequence_value_3:
        nested_sequence_key_1: nested_sequence_value_3_key_1
        nested_sequence_key_2: nested_sequence_value_3_key_2
  # bottom comment
`

	tests := []struct {
		name           string
		yamlPath       string
		expectedResult string
		expectedErr    error
	}{
		{
			name:           "It should get value from sequence by key",
			yamlPath:       "$.root.simple_key",
			expectedResult: "simple_key_value",
			expectedErr:    nil,
		},
		{
			name:           "It should get value from sequence by index",
			yamlPath:       "$.root.simple_sequence[2]",
			expectedResult: "simple_sequence_value_3",
			expectedErr:    nil,
		},
		{
			name:           "It should get value by complex path with mixed nesting",
			yamlPath:       "$.root.nested_sequence[1].nested_sequence_value_2.nested_sequence_key_2",
			expectedResult: "nested_sequence_value_2_key_2",
			expectedErr:    nil,
		},
		{
			name:        "It should return error on get if simple key not exists",
			yamlPath:    "$.root.not_existed_simple_key",
			expectedErr: errors.New("$.root.not_existed_simple_key there is no value by key"),
		},
		{
			name:        "It should return error of index out of range",
			yamlPath:    "$.root.simple_sequence[3]",
			expectedErr: errors.New("$.root.simple_sequence[3] index out of sequence range"),
		},
		{
			name:        "It should return error on get map key from sequence",
			yamlPath:    "$.root.simple_sequence.some_key",
			expectedErr: errors.New("$.root.simple_sequence mapping node expected, but found sequence node"),
		},
		{
			name:        "It should return error on get sequence node",
			yamlPath:    "$.root.simple_sequence",
			expectedErr: errors.New("$.root.simple_sequence scalar node is expected, but found mapping node"),
		},
		{
			name:        "It should return error on get mapping node",
			yamlPath:    "$.root",
			expectedErr: errors.New("$.root scalar node is expected, but found mapping node"),
		},
		{
			name:        "It should return error on root",
			yamlPath:    "$",
			expectedErr: errors.New("$ could not get scalar of root"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var data yaml.Node
			err := yaml.Unmarshal([]byte(sourceYaml), &data)
			if err != nil {
				t.Errorf("Unexpected unmarshal error: %s", err.Error())
				return
			}
			yamlPath, err := ParseYmlPath(test.yamlPath)
			if err != nil {
				t.Errorf("Unexpected parse error: %s", err.Error())
				return
			}

			// ACT
			result, err := yamlPath.GetScalarValueFrom(&data)

			// ASSERT
			if test.expectedErr != nil && err == nil {
				t.Errorf("Expected error \"%s\", but got nil", test.expectedErr)
				return
			}

			if test.expectedErr != nil && err != nil && test.expectedErr.Error() != err.Error() {
				t.Errorf("Expected \"%s\", but got \"%s\"", test.expectedErr, err)
				return
			}

			if result != test.expectedResult {
				t.Errorf("Expected \"%s\", but got \"%s\"", test.expectedResult, result)
				return
			}
		})
	}
}

func Test_Yaml3_SetScalarValueTo(t *testing.T) {
	sourceYaml := `root:
    # top comment
    simple_key: simple_key_value # simple key comment
    simple_sequence:
        # simple sequence comment
        - simple_sequence_value_1
        - simple_sequence_value_2
        - simple_sequence_value_3
    # bottom comment
`

	tests := []struct {
		name           string
		yamlPath       string
		newValue       string
		valueStyle     yaml.Style
		expectedResult string
		expectedErr    error
	}{
		{
			name:       "It should set value by key and keep comments as it is",
			yamlPath:   "$.root.simple_key",
			newValue:   "new_simple_val",
			valueStyle: yaml.FlowStyle,
			expectedResult: `root:
    # top comment
    simple_key: new_simple_val # simple key comment
    simple_sequence:
        # simple sequence comment
        - simple_sequence_value_1
        - simple_sequence_value_2
        - simple_sequence_value_3
    # bottom comment
`,
			expectedErr: nil,
		},
		{
			name:       "It should set value by key and keep comments as it is",
			yamlPath:   "$.root.simple_key",
			newValue:   "new_simple_val",
			valueStyle: yaml.FlowStyle,
			expectedResult: `root:
    # top comment
    simple_key: new_simple_val # simple key comment
    simple_sequence:
        # simple sequence comment
        - simple_sequence_value_1
        - simple_sequence_value_2
        - simple_sequence_value_3
    # bottom comment
`,
			expectedErr: nil,
		},
		{
			name:       "It should set value as double quoted string and keep comments as it is",
			yamlPath:   "$.root.simple_key",
			newValue:   "new_simple_val",
			valueStyle: yaml.DoubleQuotedStyle,
			expectedResult: `root:
    # top comment
    simple_key: "new_simple_val" # simple key comment
    simple_sequence:
        # simple sequence comment
        - simple_sequence_value_1
        - simple_sequence_value_2
        - simple_sequence_value_3
    # bottom comment
`,
			expectedErr: nil,
		},
		{
			name:        "It should fails set root value",
			yamlPath:    "$",
			newValue:    "new_simple_val",
			valueStyle:  yaml.FlowStyle,
			expectedErr: errors.New("$ could not set scalar of root"),
		},
		{
			name:        "It should fails set sequence value",
			yamlPath:    "$.root.simple_sequence",
			newValue:    "new_simple_val",
			valueStyle:  yaml.FlowStyle,
			expectedErr: errors.New("$.root.simple_sequence scalar node is expected, but found mapping node"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var data yaml.Node
			err := yaml.Unmarshal([]byte(sourceYaml), &data)
			if err != nil {
				t.Errorf("Unexpected unmarshal error: %s", err.Error())
				return
			}
			yamlPath, err := ParseYmlPath(test.yamlPath)
			if err != nil {
				t.Errorf("Unexpected parse error: %s", err.Error())
				return
			}

			// ACT
			err = yamlPath.SetScalarValueTo(&data, test.newValue, test.valueStyle)

			// ASSERT
			if test.expectedErr != nil && err == nil {
				t.Errorf("Expected error \"%s\", but got nil", test.expectedErr)
				return
			}

			if test.expectedErr != nil && err != nil && test.expectedErr.Error() != err.Error() {
				t.Errorf("Expected \"%s\", but got \"%s\"", test.expectedErr, err)
				return
			}

			updatedYmlBytes, err := yaml.Marshal(data.Content[0])
			if err != nil {
				t.Errorf("Unexpected marshal error: %s", err.Error())
				return
			}

			if test.expectedResult != "" && string(updatedYmlBytes) != test.expectedResult {
				diff := getLinesDiff(test.expectedResult, string(updatedYmlBytes))
				t.Errorf("Expected not equals updated, diff %s", diff)
				return
			}
		})
	}
}
