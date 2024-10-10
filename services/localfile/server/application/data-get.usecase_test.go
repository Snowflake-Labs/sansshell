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
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/localfile/server/infrastructure/output/file-data"
	"testing"
)

func Test__dataGetUsecase__Run(t *testing.T) {
	invalidFilepathTests := []struct {
		name            string
		invalidFilePath string
		expectedError   string
	}{
		{
			name:            "It should fail if open file path provided",
			invalidFilePath: "/not-clear/file/path/",
			expectedError:   "[1]: invalid file path: rpc error: code = InvalidArgument desc = /not-clear/file/path/ must be a clean path",
		},
		{
			name:            "It should fail if relative file path provided",
			invalidFilePath: "./relative/path",
			expectedError:   "[1]: invalid file path: rpc error: code = InvalidArgument desc = ./relative/path must be an absolute path",
		},
		{
			name:            "It should fail if parent directory relative file path provided",
			invalidFilePath: "../parent-dir/relative/path",
			expectedError:   "[1]: invalid file path: rpc error: code = InvalidArgument desc = ../parent-dir/relative/path must be an absolute path",
		},
	}

	for _, test := range invalidFilepathTests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			usecase := &dataGetUsecase{}
			ctx := context.Background()

			// ACT
			_, err := usecase.Run(ctx, test.invalidFilePath, "dataKey", pb.FileFormat_YML)

			// ASSERT
			if err == nil {
				t.Errorf("Expected error, got nil")
			}

			if err.Error() != test.expectedError {
				t.Errorf("Expected error %s, got %s", test.expectedError, err.Error())
			}
		})
	}

	t.Run("It should fail if unsupported file format provided", func(t *testing.T) {
		// ARRANGE
		instanceMap := make(map[pb.FileFormat]file_data.FileDataRepository)
		repoFactoryMock := NewFileDataRepositoryFactoryMock(instanceMap)
		usecase := &dataGetUsecase{
			fileDataRepoFactory: repoFactoryMock,
		}
		ctx := context.Background()

		// ACT
		_, err := usecase.Run(ctx, "/some/file/path", "dataKey", pb.FileFormat_UNKNOWN)

		// ASSERT
		if err == nil {
			t.Errorf("Expected error, got nil")
		}
	})

	t.Run("It should fail if unsupported file format provided", func(t *testing.T) {
		// ARRANGE
		instanceMap := make(map[pb.FileFormat]file_data.FileDataRepository)
		repoFactoryMock := NewFileDataRepositoryFactoryMock(instanceMap)
		usecase := &dataGetUsecase{
			fileDataRepoFactory: repoFactoryMock,
		}
		ctx := context.Background()

		// ACT
		_, err := usecase.Run(ctx, "/some/file/path", "dataKey", pb.FileFormat_UNKNOWN)

		// ASSERT
		if err == nil {
			t.Errorf("Expected error, got nil")
			return
		}
	})

	t.Run("It should fail if file repo return error", func(t *testing.T) {
		// ARRANGE
		dataMap := make(map[string]map[string]string)
		keyError := "some error"
		expectedError := "[3]: could not get data by \"dataKey\" key: some error"
		filePath := "/some/file/path"
		dataKey := "dataKey"
		errorOnGetKey := make(map[string]map[string]string)
		errorOnGetKey[filePath] = make(map[string]string)
		errorOnGetKey[filePath][dataKey] = keyError
		repoMock := NewFileDataRepositoryMock(dataMap, nil, errorOnGetKey)

		instanceMap := make(map[pb.FileFormat]file_data.FileDataRepository)
		instanceMap[pb.FileFormat_YML] = repoMock
		repoFactoryMock := NewFileDataRepositoryFactoryMock(instanceMap)
		usecase := &dataGetUsecase{
			fileDataRepoFactory: repoFactoryMock,
		}
		ctx := context.Background()

		// ACT
		_, err := usecase.Run(ctx, filePath, dataKey, pb.FileFormat_YML)

		// ASSERT
		if err == nil {
			t.Errorf("Expected error, got nil")
			return
		}

		if err.Error() != expectedError {
			t.Errorf("Expected error \"%s\", got \"%s\"", expectedError, err.Error())
			return
		}
	})

	t.Run("It should return data by key, if repo return data", func(t *testing.T) {
		// ARRANGE
		filePath := "/some/file/path"
		dataKey := "dataKey"
		expectedData := "some data"
		dataMap := make(map[string]map[string]string)
		dataMap[filePath] = make(map[string]string)
		dataMap[filePath][dataKey] = expectedData
		repoMock := NewFileDataRepositoryMock(dataMap, nil, nil)

		instanceMap := make(map[pb.FileFormat]file_data.FileDataRepository)
		instanceMap[pb.FileFormat_YML] = repoMock
		repoFactoryMock := NewFileDataRepositoryFactoryMock(instanceMap)
		usecase := &dataGetUsecase{
			fileDataRepoFactory: repoFactoryMock,
		}
		ctx := context.Background()

		// ACT
		data, err := usecase.Run(ctx, filePath, dataKey, pb.FileFormat_YML)

		// ASSERT
		if err != nil {
			t.Errorf("Unexpected error: %s", err.Error())
			return
		}

		if data != expectedData {
			t.Errorf("Expected data %s, got %s", expectedData, data)
			return
		}
	})

}
