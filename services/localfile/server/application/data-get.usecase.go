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

package application

import (
	"context"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/localfile/server/infrastructure/output/file-data"
	"github.com/Snowflake-Labs/sansshell/services/util"
	error_utils "github.com/Snowflake-Labs/sansshell/services/util/error-utils"
	"github.com/go-logr/logr"
)

type DataGetErrorCodes int

var (
	// DataGetErrorCodes_FilePathInvalid error code for invalid file path was provided
	DataGetErrorCodes_FilePathInvalid DataGetErrorCodes = 1
	// DataGetErrorCodes_FileFormatNotSupported error code for file format not supported
	DataGetErrorCodes_FileFormatNotSupported DataGetErrorCodes = 2
	// DataGetErrorCodes_CouldNotGetData error code for could not get data by provided key
	DataGetErrorCodes_CouldNotGetData DataGetErrorCodes = 3
)

// DataGetUsecase usecase interface for getting data from file of specific format by provided data key
type DataGetUsecase interface {
	// Run gets data from file by provided data key
	//   Returns data value or [error_utils.ErrorWithCode[DataGetErrorCodes]] if error occurred
	Run(ctx context.Context, filePath string, dataKey string, fileFormat pb.FileFormat) (string, error_utils.ErrorWithCode[DataGetErrorCodes])
}

// NewDataGetUsecase creates new instance of [DataSetUsecase]
func NewDataGetUsecase(fileDataReaderFactory file_data.FileDataRepositoryFactory) DataGetUsecase {
	return &dataGetUsecase{
		fileDataRepoFactory: fileDataReaderFactory,
	}
}

// dataGetUsecase implementation of [DataSetUsecase] interface
type dataGetUsecase struct {
	fileDataRepoFactory file_data.FileDataRepositoryFactory
}

// Run implementation of [DataSetUsecase.Run] interface
func (u *dataGetUsecase) Run(ctx context.Context, filePath string, dataKey string, fileFormat pb.FileFormat) (string, error_utils.ErrorWithCode[DataGetErrorCodes]) {
	logger := logr.FromContextOrDiscard(ctx)
	err := util.ValidPath(filePath)
	if err != nil {
		logger.Error(err, "invalid file path")
		return "", error_utils.NewErrorWithCodef(DataGetErrorCodes_FilePathInvalid, "invalid file path: %s", err.Error())
	}

	fileDataRepo, err := u.fileDataRepoFactory.GetRepository(fileFormat)
	if err != nil {
		logger.Error(err, "unsupported file format", "format name", fileFormat.String(), "file path", filePath)
		return "", error_utils.NewErrorWithCodef(DataGetErrorCodes_FileFormatNotSupported, "file fromat is not supported \"%s\" format", fileFormat.String())
	}

	value, err := fileDataRepo.GetDataByKey(filePath, dataKey)
	if err != nil {
		return "", error_utils.NewErrorWithCodef(DataGetErrorCodes_CouldNotGetData, "could not get data by \"%s\" key: %s", dataKey, err.Error())
	}

	return value, nil
}
