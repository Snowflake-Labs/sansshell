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
)

// DataGetUsecase usecase interface for getting data from file of specific format
type DataGetUsecase interface {
	// Run runs usecase
	Run(ctx context.Context, remoteFilePath string, dataKey string, fileFormat pb.FileFormat) (<-chan *pb.DataGetManyResponse, error)
}

// NewDataGetUsecase creates new instance of DataGetUsecase
func NewDataGetUsecase(client pb.LocalFileClientProxy) DataGetUsecase {
	return &dataGetUsecase{
		client: client,
	}
}

// dataGetUsecase implementation of [DataGetUsecase] interface
type dataGetUsecase struct {
	client pb.LocalFileClientProxy
}

func (u *dataGetUsecase) Run(ctx context.Context, remoteFilePath string, dataKey string, fileFormat pb.FileFormat) (<-chan *pb.DataGetManyResponse, error) {
	responses, err := u.client.DataGetOneMany(ctx, &pb.DataGetRequest{
		Filename:   remoteFilePath,
		DataKey:    dataKey,
		FileFormat: fileFormat,
	})

	return responses, err
}
