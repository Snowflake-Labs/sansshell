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

package application

import (
	"context"
	"errors"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/localfile/server/infrastructure/output/file-data"
)

// -----------------------------------------------------------------------------
// FileDataRepositoryFactoryMock implementation
// -----------------------------------------------------------------------------
func NewFileDataRepositoryFactoryMock(instanceMap map[pb.FileFormat]file_data.FileDataRepository) file_data.FileDataRepositoryFactory {
	return &fileDataRepositoryFactoryMock{
		instanceMap: instanceMap,
	}
}

type fileDataRepositoryFactoryMock struct {
	instanceMap map[pb.FileFormat]file_data.FileDataRepository
}

func (f *fileDataRepositoryFactoryMock) GetRepository(ctx context.Context, fileFormat pb.FileFormat) (file_data.FileDataRepository, error) {
	if instanceMap, ok := f.instanceMap[fileFormat]; ok {
		return instanceMap, nil
	}

	return nil, errors.New("unsupported file type")
}

// -----------------------------------------------------------------------------
// FileDataRepositoryMock implementation
// -----------------------------------------------------------------------------
func NewFileDataRepositoryMock(dataByFileByKey map[string]map[string]string, errorOnSetKey map[string]map[string]string, errorOnGetKey map[string]map[string]string) file_data.FileDataRepository {
	return &fileDataRepositoryMock{
		dataByKey:     dataByFileByKey,
		errorOnSetKey: errorOnSetKey,
		errorOnGetKey: errorOnGetKey,
	}
}

type fileDataRepositoryMock struct {
	dataByKey     map[string]map[string]string
	errorOnSetKey map[string]map[string]string
	errorOnGetKey map[string]map[string]string
}

func (f *fileDataRepositoryMock) GetDataByKey(filePath string, key string) (string, error) {
	if val, ok := f.errorOnGetKey[filePath][key]; ok {
		return "", errors.New(val)
	}

	if value, ok := f.dataByKey[filePath][key]; ok {
		return value, nil
	}

	return "", errors.New("key not found")
}

func (f *fileDataRepositoryMock) SetDataByKey(filePath string, key string, value string, valueType pb.DataSetValueType) error {
	if val, ok := f.errorOnSetKey[filePath][key]; ok {
		return errors.New(val)
	}

	if _, ok := f.dataByKey[filePath][key]; !ok {
		return errors.New("key not found")
	}

	f.dataByKey[filePath][key] = value
	return nil
}
