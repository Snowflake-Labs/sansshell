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
package file_data

import (
	"context"
	"errors"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_FileDataDonEnvRepository_GetDataByKey(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test")
	}

	validDotEnv := `SOME=VAR
SOME_OTHER=VAR_VAL
`

	validYmlTests := []struct {
		name           string
		key            string
		expectedResult string
		expectedErr    error
	}{
		{
			name:           "It should get value by key",
			key:            "SOME_OTHER",
			expectedResult: "VAR_VAL",
			expectedErr:    nil,
		},
		{
			name:        "It should get error if key not found",
			key:         "NOT_EXISTED_KEY",
			expectedErr: errors.New("key \"NOT_EXISTED_KEY\" not found"),
		},
	}

	for _, test := range validYmlTests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			repo := &fileDataDotEnvRepository{}
			release, filePath, err := writeStringToTmpFile(t, "test.yml", validDotEnv)
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
		repo := &fileDataDotEnvRepository{}
		expectedError := "failed to read file"

		// ACT
		_, err := repo.GetDataByKey("not_existed_file.env", "SOME_KEY")

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

	t.Run("It should fail if file contains not valid dotenv", func(t *testing.T) {
		// ARRANGE
		repo := &fileDataDotEnvRepository{}
		yml := "^INVALID VAR=VALUE"
		release, filePath, err := writeStringToTmpFile(t, "test.env", yml)
		if err != nil {
			t.Errorf("Unexpected tmp file creation error: %s", err.Error())
			return
		}
		defer (func() {
			_ = release()
		})()
		expectedError := "failed to read file"

		// ACT
		_, err = repo.GetDataByKey(filePath, "VAR")

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

func Test_FileDataDonEnvRepository_SetDataByKey(t *testing.T) {
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

// writeDotEnvTmpFile writes content to a throwaway .env file in a per-test temp
// directory and returns its path.
func writeDotEnvTmpFile(t *testing.T, content string) string {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), "test.env")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write tmp file: %s", err)
	}
	return filePath
}

// TestIntegration_FileDataDotEnvRepository_SetDataByKey_RejectsInjection is the
// CWE-93 regression suite: a caller authorized for a single dotenv key must
// never be able to inject additional environment variables via a newline (or
// an unsafe key), and a rejected write must not modify the file.
func TestIntegration_FileDataDotEnvRepository_SetDataByKey_RejectsInjection(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test")
	}

	const initialContent = "SOME=VAR\n"

	rejectionTests := []struct {
		name           string
		key            string
		value          string
		expectedErrSub string
	}{
		{
			name:           "It should reject a newline-injected value (LD_PRELOAD PoC)",
			key:            "SOME",
			value:          "info\nLD_PRELOAD=/tmp/evil.so",
			expectedErrSub: "newline character",
		},
		{
			name:           "It should reject a carriage-return-injected value",
			key:            "SOME",
			value:          "info\rLD_PRELOAD=/tmp/evil.so",
			expectedErrSub: "newline character",
		},
		{
			name:           "It should reject a CRLF-injected value",
			key:            "SOME",
			value:          "info\r\nLD_PRELOAD=/tmp/evil.so",
			expectedErrSub: "newline character",
		},
		{
			name:           "It should reject other control characters in a value",
			key:            "SOME",
			value:          "info\x00evil",
			expectedErrSub: "control character",
		},
		{
			name:           "It should reject a newline-injected key",
			key:            "SOME\nLD_PRELOAD",
			value:          "info",
			expectedErrSub: "invalid data key",
		},
		{
			name:           "It should reject a key containing an equals sign",
			key:            "SOME=LD_PRELOAD",
			value:          "info",
			expectedErrSub: "invalid data key",
		},
		{
			name:           "It should reject a key with a leading digit",
			key:            "1SOME",
			value:          "info",
			expectedErrSub: "invalid data key",
		},
		{
			name:           "It should reject a key with a dot",
			key:            "SOME.KEY",
			value:          "info",
			expectedErrSub: "invalid data key",
		},
		{
			name:           "It should reject a key with a space",
			key:            "SOME KEY",
			value:          "info",
			expectedErrSub: "invalid data key",
		},
	}

	for _, test := range rejectionTests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			repo := newDotEnvFileDataRepository(context.Background())
			filePath := writeDotEnvTmpFile(t, initialContent)

			// ACT
			err := repo.SetDataByKey(filePath, test.key, test.value, pb.DataSetValueType_STRING_VAL)

			// ASSERT: the write is rejected ...
			if err == nil {
				t.Fatalf("Expected error, but got nil")
			}
			if !strings.Contains(err.Error(), test.expectedErrSub) {
				t.Fatalf("Expected error containing %q, but got %q", test.expectedErrSub, err.Error())
			}

			// ... and the file is left untouched (writes nothing).
			after, readErr := os.ReadFile(filePath)
			if readErr != nil {
				t.Fatalf("failed to read file after rejected write: %s", readErr)
			}
			if string(after) != initialContent {
				t.Fatalf("Expected file to be unchanged %q, but got %q", initialContent, string(after))
			}

			// ... and no injected variable is observable by a dotenv consumer.
			envMap, readErr := godotenv.Read(filePath)
			if readErr != nil {
				t.Fatalf("failed to parse file after rejected write: %s", readErr)
			}
			if _, injected := envMap["LD_PRELOAD"]; injected {
				t.Fatalf("LD_PRELOAD was injected into the .env file: %#v", envMap)
			}
			if len(envMap) != 1 {
				t.Fatalf("Expected exactly one variable after rejected write, got %#v", envMap)
			}
		})
	}

	t.Run("It should keep a single physical line across a two-call rewrite attempt", func(t *testing.T) {
		// ARRANGE
		repo := newDotEnvFileDataRepository(context.Background())
		filePath := writeDotEnvTmpFile(t, initialContent)

		// ACT 1: a legitimate write for the authorized key succeeds.
		if err := repo.SetDataByKey(filePath, "SOME", "hello", pb.DataSetValueType_STRING_VAL); err != nil {
			t.Fatalf("Unexpected error on legitimate write: %s", err)
		}

		// ACT 2: an injection attempt on the same key is rejected.
		err := repo.SetDataByKey(filePath, "SOME", "world\nLD_PRELOAD=/tmp/evil.so", pb.DataSetValueType_STRING_VAL)
		if err == nil {
			t.Fatalf("Expected error on injection attempt, but got nil")
		}

		// ASSERT: only the authorized key exists, with its last legitimate value.
		envMap, readErr := godotenv.Read(filePath)
		if readErr != nil {
			t.Fatalf("failed to parse file: %s", readErr)
		}
		if _, injected := envMap["LD_PRELOAD"]; injected {
			t.Fatalf("LD_PRELOAD was injected: %#v", envMap)
		}
		if got := envMap["SOME"]; got != "hello" {
			t.Fatalf("Expected SOME=hello, got SOME=%q", got)
		}
		if len(envMap) != 1 {
			t.Fatalf("Expected exactly one variable, got %#v", envMap)
		}
	})
}

// TestIntegration_FileDataDotEnvRepository_SetDataByKey_HappyPath verifies the
// guard does not break legitimate writes (overwrite and append of safe keys/values).
func TestIntegration_FileDataDotEnvRepository_SetDataByKey_HappyPath(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test")
	}

	t.Run("It should overwrite an existing key", func(t *testing.T) {
		repo := newDotEnvFileDataRepository(context.Background())
		filePath := writeDotEnvTmpFile(t, "SOME=VAR\n")

		if err := repo.SetDataByKey(filePath, "SOME", "changed", pb.DataSetValueType_STRING_VAL); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		envMap, err := godotenv.Read(filePath)
		if err != nil {
			t.Fatalf("failed to parse file: %s", err)
		}
		if got := envMap["SOME"]; got != "changed" {
			t.Fatalf("Expected SOME=changed, got SOME=%q", got)
		}
	})

	t.Run("It should append a new key", func(t *testing.T) {
		repo := newDotEnvFileDataRepository(context.Background())
		filePath := writeDotEnvTmpFile(t, "SOME=VAR\n")

		if err := repo.SetDataByKey(filePath, "NEW_KEY", "new_val", pb.DataSetValueType_STRING_VAL); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		envMap, err := godotenv.Read(filePath)
		if err != nil {
			t.Fatalf("failed to parse file: %s", err)
		}
		if got := envMap["SOME"]; got != "VAR" {
			t.Fatalf("Expected existing SOME=VAR to be preserved, got SOME=%q", got)
		}
		if got := envMap["NEW_KEY"]; got != "new_val" {
			t.Fatalf("Expected NEW_KEY=new_val, got NEW_KEY=%q", got)
		}
	})
}
