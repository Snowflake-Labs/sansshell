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
	"fmt"
	"os"
	"strings"
	"testing"
)

func writeStringToTmpFile(t *testing.T, fileName string, content string) (func() error, string, error) {
	testName := strings.ReplaceAll(t.Name(), " ", "_")
	testName = strings.ReplaceAll(testName, "/", "__")
	filePath := fmt.Sprintf("./__testdata__/%s__%s", testName, fileName)
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %s", err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return nil, "", fmt.Errorf("failed to write to file: %s", err)
	}

	return func() error {
		return os.Remove(filePath)
	}, filePath, nil
}

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
