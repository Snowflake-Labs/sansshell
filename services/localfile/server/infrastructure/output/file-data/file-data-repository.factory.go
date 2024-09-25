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
	"errors"
)
import pb "github.com/Snowflake-Labs/sansshell/services/localfile"

// FileDataRepository is an interface to read data from file by key
type FileDataRepository interface {
	// GetDataByKey Read data from file by key
	GetDataByKey(filePath string, key string) (string, error)
	// SetDataByKey Set data to file by key
	SetDataByKey(filePath string, key string, value string, valType pb.DataSetValueType) error
}

// FileDataRepositoryFactory is an interface to get a new [FileDataRepository] by file format
type FileDataRepositoryFactory interface {
	// GetRepository Get [FileDataRepository] by file format
	GetRepository(context context.Context, fileFormat pb.FileFormat) (FileDataRepository, error)
}

// NewFileDataRepositoryFactory creates a new instance of [application.FileDataRepositoryFactory]
func NewFileDataRepositoryFactory() FileDataRepositoryFactory {
	return &fileDataReaderFactory{}
}

// fileDataReaderFactory implementation of [application.FileDataRepositoryFactory] interface
type fileDataReaderFactory struct {
}

// GetRepository implementation of [application.FileDataRepositoryFactory.GetRepository] interface
func (f *fileDataReaderFactory) GetRepository(context context.Context, fileFormat pb.FileFormat) (FileDataRepository, error) {
	switch fileFormat {
	case pb.FileFormat_YML:
		return newFileDataYmlRepository(context), nil
	case pb.FileFormat_DOTENV:
		return newDotEnvFileDataRepository(context), nil
	default:

		return nil, errors.New("Unsupported file type")
	}
}
