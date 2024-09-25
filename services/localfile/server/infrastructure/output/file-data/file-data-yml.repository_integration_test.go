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
package file_data

import (
	"errors"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"os"
	"testing"
)

func Test_FileDataYmlRepository_GetDataByKey(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test")
	}

	validYml := `
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

	validYmlTests := []struct {
		name           string
		key            string
		expectedResult string
		expectedErr    error
	}{
		{
			name:        "It should fail in case of invalid path",
			key:         "$.root.[[0]",
			expectedErr: errors.New("failed to parse key: root.[[0] invalid path, too many \"[\" in key"),
		},
		{
			name:           "It should get value from sequence by key",
			key:            "$.root.simple_key",
			expectedResult: "simple_key_value",
			expectedErr:    nil,
		},
		{
			name:           "It should get value from sequence by index",
			key:            "$.root.simple_sequence[2]",
			expectedResult: "simple_sequence_value_3",
			expectedErr:    nil,
		},
		{
			name:           "It should get value by complex path with mixed nesting",
			key:            "$.root.nested_sequence[1].nested_sequence_value_2.nested_sequence_key_2",
			expectedResult: "nested_sequence_value_2_key_2",
			expectedErr:    nil,
		},
		{
			name:        "It should return error on get if simple key not exists",
			key:         "$.root.not_existed_simple_key",
			expectedErr: errors.New("failed to get value: $.root.not_existed_simple_key there is no value by key"),
		},
		{
			name:        "It should return error of index out of range",
			key:         "$.root.simple_sequence[3]",
			expectedErr: errors.New("failed to get value: $.root.simple_sequence[3] index out of sequence range"),
		},
		{
			name:        "It should return error on get map key from sequence",
			key:         "$.root.simple_sequence.some_key",
			expectedErr: errors.New("failed to get value: $.root.simple_sequence mapping node expected, but found sequence node"),
		},
		{
			name:        "It should return error on get sequence node",
			key:         "$.root.simple_sequence",
			expectedErr: errors.New("failed to get value: $.root.simple_sequence scalar node is expected, but found mapping node"),
		},
		{
			name:        "It should return error on get mapping node",
			key:         "$.root",
			expectedErr: errors.New("failed to get value: $.root scalar node is expected, but found mapping node"),
		},
		{
			name:        "It should return error on root",
			key:         "$",
			expectedErr: errors.New("failed to get value: $ could not get scalar of root"),
		},
	}

	for _, test := range validYmlTests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			repo := &fileDataYmlRepository{}
			release, filePath, err := writeStringToTmpFile(t, "test.yml", validYml)
			if err != nil {
				t.Errorf("Unexpected tmp file creation error: %s", err.Error())
				return
			}
			defer (func() {
				_ = release()
			})()

			// ACT
			result, err := repo.GetDataByKey(filePath, test.key)

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

	t.Run("It should fail if file no exists", func(t *testing.T) {
		// ARRANGE
		repo := &fileDataYmlRepository{}
		expectedError := "failed to read file content: open not_existed_file.yml: no such file or directory"

		// ACT
		_, err := repo.GetDataByKey("not_existed_file.yml", "$.root.simple_key")

		// ASSERT
		if err == nil {
			t.Errorf("Expected error, but got nil")
			return
		}

		if err.Error() != expectedError {
			t.Errorf("Expected \"%s\", but got \"%s\"", expectedError, err.Error())
			return
		}
	})

	t.Run("It should fail if file contains not valid yml", func(t *testing.T) {
		// ARRANGE
		repo := &fileDataYmlRepository{}
		yml := "@some: not valid yml"
		release, filePath, err := writeStringToTmpFile(t, "test.yml", yml)
		if err != nil {
			t.Errorf("Unexpected tmp file creation error: %s", err.Error())
			return
		}
		defer (func() {
			_ = release()
		})()
		expectedError := "failed to parse file: yaml: found character that cannot start any token"

		// ACT
		_, err = repo.GetDataByKey(filePath, "$.some")

		// ASSERT
		if err == nil {
			t.Errorf("Expected error, but got nil")
			return
		}

		if err.Error() != expectedError {
			t.Errorf("Expected \"%s\", but got \"%s\"", expectedError, err.Error())
			return
		}
	})
}

func Test_FileDataYmlRepository_SetDataByKey(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test")
	}

	validSourceYaml := `root:
    # top comment
    simple_key: simple_key_value # simple key comment
    simple_sequence:
        # simple sequence comment
        - simple_sequence_value_1
        - simple_sequence_value_2
        - simple_sequence_value_3
    # bottom comment
`

	validYmlTests := []struct {
		name           string
		yamlPath       string
		newValue       string
		valueType      pb.DataSetValueType
		expectedResult string
		expectedErr    error
	}{
		{
			name:      "It should set value by key and keep comments as it is",
			yamlPath:  "$.root.simple_key",
			newValue:  "newval",
			valueType: pb.DataSetValueType_STRING_VAL,
			expectedResult: `root:
    # top comment
    simple_key: newval # simple key comment
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
			name:      "It should set int value by key and keep comments as it is",
			yamlPath:  "$.root.simple_key",
			newValue:  "12",
			valueType: pb.DataSetValueType_INT_VAL,
			expectedResult: `root:
    # top comment
    simple_key: "12" # simple key comment
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
			name:      "It should set float value by key and keep comments as it is",
			yamlPath:  "$.root.simple_key",
			newValue:  "12.12",
			valueType: pb.DataSetValueType_FLOAT_VAL,
			expectedResult: `root:
    # top comment
    simple_key: "12.12" # simple key comment
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
			name:      "It should set bool value by key and keep comments as it is",
			yamlPath:  "$.root.simple_key",
			newValue:  "false",
			valueType: pb.DataSetValueType_BOOL_VAL,
			expectedResult: `root:
    # top comment
    simple_key: "False" # simple key comment
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
			name:      "It should set string value as double quoted string and keep comments as it is",
			yamlPath:  "$.root.simple_key",
			newValue:  "new simple val",
			valueType: pb.DataSetValueType_STRING_VAL,
			expectedResult: `root:
    # top comment
    simple_key: "new simple val" # simple key comment
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
			valueType:   pb.DataSetValueType_STRING_VAL,
			expectedErr: errors.New("failed to set value: $ could not set scalar of root"),
		},
		{
			name:        "It should fails set sequence value",
			yamlPath:    "$.root.simple_sequence",
			newValue:    "new_simple_val",
			valueType:   pb.DataSetValueType_STRING_VAL,
			expectedErr: errors.New("failed to set value: $.root.simple_sequence scalar node is expected, but found mapping node"),
		},
	}

	for _, test := range validYmlTests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			repo := &fileDataYmlRepository{}
			release, filePath, err := writeStringToTmpFile(t, "test.yml", validSourceYaml)
			if err != nil {
				t.Errorf("Unexpected tmp file creation error: %s", err.Error())
				return
			}
			defer (func() {
				_ = release()
			})()

			// ACT
			err = repo.SetDataByKey(filePath, test.yamlPath, test.newValue, test.valueType)

			// ASSERT
			if test.expectedErr != nil && err == nil {
				t.Errorf("Expected error \"%s\", but got nil", test.expectedErr)
				return
			}

			if test.expectedErr != nil && err != nil && test.expectedErr.Error() != err.Error() {
				t.Errorf("Expected \"%s\", but got \"%s\"", test.expectedErr, err)
				return
			}

			updatedYmlBytes, err := os.ReadFile(filePath)
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

	t.Run("It should fail if file contains not valid yml", func(t *testing.T) {
		// ARRANGE
		repo := &fileDataYmlRepository{}
		yml := "@some: not valid yml"
		release, filePath, err := writeStringToTmpFile(t, "test.yml", yml)
		if err != nil {
			t.Errorf("Unexpected tmp file creation error: %s", err.Error())
			return
		}
		defer (func() {
			_ = release()
		})()
		expectedError := "failed to parse yaml: yaml: found character that cannot start any token"

		// ACT
		err = repo.SetDataByKey(filePath, "$.root.simple_key", "new_val", pb.DataSetValueType_STRING_VAL)

		// ASSERT
		if err == nil {
			t.Errorf("Expected error, but got nil")
			return
		}

		if err.Error() != expectedError {
			t.Errorf("Expected \"%s\", but got \"%s\"", expectedError, err.Error())
			return
		}
	})
}
