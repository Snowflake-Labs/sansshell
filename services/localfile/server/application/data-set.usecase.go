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

type DataSetErrorCodes int

var (
	// DataSetErrorCodes_FilePathInvalid error code for invalid file path was provided
	DataSetErrorCodes_FilePathInvalid DataSetErrorCodes = 1
	// DataGetErrorCodes_FileFormatNotSupported error code for file format not supported
	DataSetErrorCodes_FileFormatNotSupported DataSetErrorCodes = 2
	// DataGetErrorCodes_CouldNotGetData error code for could not get data by provided key
	DataSetErrorCodes_CouldNotSetData DataSetErrorCodes = 3
)

// DataSetUsecase usecase interface for set data to file of specific format by provided data key
type DataSetUsecase interface {
	// Run sets data to file by provided data key
	//   Returns [error_utils.ErrorWithCode[DataSetErrorCodes]] if error occurred
	Run(ctx context.Context, filePath string, dataKey string, fileFormat pb.FileFormat, value string, valueType pb.DataSetValueType) error_utils.ErrorWithCode[DataSetErrorCodes]
}

// NewDataSetUsecase creates new instance of [DataSetUsecase]
func NewDataSetUsecase(fileDataRepoFactory file_data.FileDataRepositoryFactory) DataSetUsecase {
	return &dataSetUsecase{
		fileDataRepoFactory: fileDataRepoFactory,
	}
}

// dataSetUsecase implementation of [DataSetUsecase] interface
type dataSetUsecase struct {
	fileDataRepoFactory file_data.FileDataRepositoryFactory
}

func (u *dataSetUsecase) Run(ctx context.Context, filePath string, dataKey string, fileFormat pb.FileFormat, value string, valueType pb.DataSetValueType) error_utils.ErrorWithCode[DataSetErrorCodes] {
	logger := logr.FromContextOrDiscard(ctx)
	err := util.ValidPath(filePath)
	if err != nil {
		logger.Error(err, "invalid file path")
		return error_utils.NewErrorWithCodef(DataSetErrorCodes_FilePathInvalid, "invalid file path: %s", err.Error())
	}

	fileDataRepo, err := u.fileDataRepoFactory.GetRepository(fileFormat)
	if err != nil {
		logger.Error(err, "unsupported file format", "format name", fileFormat.String(), "file path", filePath)
		return error_utils.NewErrorWithCodef(DataSetErrorCodes_FileFormatNotSupported, "file fromat is not supported \"%s\"", fileFormat.String())
	}

	err = fileDataRepo.SetDataByKey(filePath, dataKey, value, valueType)
	if err != nil {
		return error_utils.NewErrorWithCodef(DataSetErrorCodes_CouldNotSetData, "could not set data by \"%s\" key: %s", dataKey, err.Error())
	}

	return nil
}
