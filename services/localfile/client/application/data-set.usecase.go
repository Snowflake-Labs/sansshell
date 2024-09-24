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
)

// DataSetUsecase usecase interface for getting data from file of specific format
type DataSetUsecase interface {
	// Run runs usecase
	Run(ctx context.Context, remoteFilePath string, dataKey string, fileFormat pb.FileFormat, value string, valueFormat pb.DataSetValueType) (<-chan *pb.DataSetManyResponse, error)
}

// NewDataSetUsecase creates new instance of DataSetUsecase
func NewDataSetUsecase(client pb.LocalFileClientProxy) DataSetUsecase {
	return &dataSetUsecase{
		client: client,
	}
}

// dataSetUsecase implementation of [DataSetUsecase] interface
type dataSetUsecase struct {
	client pb.LocalFileClientProxy
}

func (u *dataSetUsecase) Run(ctx context.Context, remoteFilePath string, dataKey string, fileFormat pb.FileFormat, value string, valueType pb.DataSetValueType) (<-chan *pb.DataSetManyResponse, error) {
	responses, err := u.client.DataSetOneMany(ctx, &pb.DataSetRequest{
		Filename:   remoteFilePath,
		DataKey:    dataKey,
		FileFormat: fileFormat,
		Value:      value,
		ValueType:  valueType,
	})

	return responses, err
}
