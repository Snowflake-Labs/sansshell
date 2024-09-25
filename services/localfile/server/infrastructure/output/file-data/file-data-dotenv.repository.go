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
	"context"
	"fmt"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	arrayUtils "github.com/Snowflake-Labs/sansshell/services/util/array-utils"
	fileUtils "github.com/Snowflake-Labs/sansshell/services/util/file-utils"
	"github.com/go-logr/logr"
	"github.com/joho/godotenv"
	"io"
	"strings"
)

func newDotEnvFileDataRepository(context context.Context) FileDataRepository {
	return &fileDataDotEnvRepository{
		context: context,
	}
}

// fileDataDotEnvRepository implementation of [application.FileDataRepository] interface
type fileDataDotEnvRepository struct {
	context context.Context
}

// GetDataByKey implementation of [application.FileDataRepository.GetDataByKey] interface
func (y *fileDataDotEnvRepository) GetDataByKey(filePath string, key string) (string, error) {
	envMap, err := godotenv.Read(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file")
	}

	val, exists := envMap[key]
	if !exists {
		return "", fmt.Errorf("key \"%s\" not found", key)
	}
	return val, nil
}

// SetDataByKey implementation of [application.FileDataRepository.SetDataByKey] interface
func (y *fileDataDotEnvRepository) SetDataByKey(filePath string, key string, value string, valType pb.DataSetValueType) error {
	if valType != pb.DataSetValueType_STRING_VAL {
		return fmt.Errorf("unsupported value type: %s", valType)
	}

	f, err := fileUtils.OpenForOverwrite(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %s", err)
	}
	defer f.Close() // close file when done

	// Apply an exclusive lock
	unlock, err := fileUtils.ExclusiveLockFile(f)
	if err != nil {
		return fmt.Errorf("failed to lock file: %s", err)
	}
	defer (func() {
		logger := logr.FromContextOrDiscard(y.context)
		err := unlock() // unlock the file when done
		logger.Error(err, "failed to unlock file")
	})()

	r := io.Reader(f)
	rawContent, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	content := string(rawContent)
	lines := strings.Split(content, "\n")

	varLinePrefix := key + "="
	varLineIndex := arrayUtils.FindIndexBy(lines, func(line string) bool {
		return strings.HasPrefix(line, varLinePrefix)
	})

	if varLineIndex == -1 {
		lines = append(lines, escapeDotEnvVarValue(key)+"="+escapeDotEnvVarValue(value))
	} else {
		lines[varLineIndex] = key + "=" + escapeDotEnvVarValue(value)
	}

	updatedContent := strings.Join(lines, "\n")

	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file: %s", err)
	}

	if _, err := f.WriteString(updatedContent); err != nil {
		return fmt.Errorf("failed to write file: %s", err)
	}

	return nil
}

var doubleQuoteSpecialChars = map[string]string{
	"\\": "\\\\",
	"\n": "\\\n",
	"\r": "\\\r",
	"\"": "\\\"",
	"!":  "\\!",
	"$":  "\\$",
	"`":  "\\`",
}

func escapeDotEnvVarValue(value string) string {
	for source, toReplace := range doubleQuoteSpecialChars {
		value = strings.Replace(value, source, toReplace, -1)
	}
	return value
}
