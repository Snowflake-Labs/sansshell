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
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	fileUtils "github.com/Snowflake-Labs/sansshell/services/util/file-utils"
	stringUtils "github.com/Snowflake-Labs/sansshell/services/util/string-utils"
	yaml3Utils "github.com/Snowflake-Labs/sansshell/services/util/yml-utils"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"strings"
)

// NewFileDataYmlRepository creates a new instance of [application.FileDataRepository]
func newFileDataYmlRepository() FileDataRepository {
	return &FileDataYmlRepository{}
}

// FileDataYmlRepository implementation of [application.FileDataRepository] interface
type FileDataYmlRepository struct {
}

// GetDataByKey implementation of [application.FileDataRepository.GetDataByKey] interface
func (y *FileDataYmlRepository) GetDataByKey(filePath string, key string) (string, error) {
	path, err := yaml3Utils.ParseYmlPath(key)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse key")
	}

	rawYml, err := os.ReadFile(filePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read file content")
	}

	// Parse YAML into a map
	var data yaml.Node
	err = yaml.Unmarshal(rawYml, &data)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse file")
	}

	val, err := path.GetScalarValueFrom(&data)
	if err != nil {
		return "", fmt.Errorf("failed to get value: %s", err)
	}

	return val, nil
}

// SetDataByKey implementation of [application.FileDataRepository.SetDataByKey] interface
func (y *FileDataYmlRepository) SetDataByKey(filePath string, key string, value string, valType pb.DataSetValueType) error {
	path, err := yaml3Utils.ParseYmlPath(key)
	if err != nil {
		return fmt.Errorf("failed to parse path: %s", err)
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

	defer unlock() // unlock the file when done

	reader := io.Reader(f)
	yamlFile, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read yaml file: %s", err)
	}

	// Parse YAML into a map
	var data yaml.Node
	err = yaml.Unmarshal(yamlFile, &data)
	if err != nil {
		return fmt.Errorf("failed to parse yaml: %s", err)
	}

	ymlVal, ymlStyle, err := toYamlVal(value, valType)
	if err != nil {
		return fmt.Errorf("failed to convert value: %s", err)
	}

	err = path.SetScalarValueTo(&data, ymlVal, ymlStyle)
	if err != nil {
		return fmt.Errorf("failed to set value: %s", err)
	}

	// Marshal the modified data back to YAML
	modifiedYAML, err := yaml.Marshal(data.Content[0])
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %s", err)
	}

	// Clear the file content
	err = f.Truncate(0)
	if err != nil {
		return fmt.Errorf("failed to truncate file: %s", err)
	}

	// Move the file pointer to the beginning
	_, err = f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek file: %s", err)
	}

	// write update content
	_, err = f.Write(modifiedYAML)
	if err != nil {
		return fmt.Errorf("failed to write to file: %s", err)
	}

	// Sync the file to disk
	err = f.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync file: %s", err)
	}

	return nil
}

func toYamlVal(val string, t pb.DataSetValueType) (string, yaml.Style, error) {
	switch t {
	case pb.DataSetValueType_BOOL_VAL:
		if strings.ToLower(val) == "true" {
			return "True", yaml.FlowStyle, nil
		}

		if strings.ToLower(val) == "false" {
			return "False", yaml.FlowStyle, nil
		}

		return "", 0, fmt.Errorf("invalid boolean value only \"true\" or \"false\" is expected")
	case pb.DataSetValueType_INT_VAL, pb.DataSetValueType_FLOAT_VAL:
		return val, yaml.FlowStyle, nil
	case pb.DataSetValueType_STRING_VAL:
		if stringUtils.IsAlphanumeric(val) == false {
			return val, yaml.DoubleQuotedStyle, nil
		}

		return val, yaml.FlowStyle, nil
	}

	return "", 0, fmt.Errorf("unsupported value type: %s", t.String())
}
