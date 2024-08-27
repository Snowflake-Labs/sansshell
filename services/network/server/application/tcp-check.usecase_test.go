/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

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
	"testing"
)

func TestTCPCheckUsecase_Run(t *testing.T) {
	t.Run("It should return ok if host available", func(t *testing.T) {
		// ARRANGE
		availableHostsPorts := []string{"localhost:80"}
		unexpectedErrorHosts := []string{}
		clientMock := NewTCPClientPortMock(availableHostsPorts, unexpectedErrorHosts)
		usecase := NewTCPCheckUsecase(clientMock)

		// ACT
		result, err := usecase.Run(context.Background(), "localhost", 80, 1)

		// ASSERT
		if err != nil {
			t.Errorf("Unexpected error: %s", err.Error())
			return
		}

		if !result.IsOk {
			t.Errorf("Unexpected status: %t", result.IsOk)
		}
	})

	t.Run("It should return NOT ok if host is unavailable", func(t *testing.T) {
		// ARRANGE
		availableHostsPorts := []string{}
		unexpectedErrorHosts := []string{}
		clientMock := NewTCPClientPortMock(availableHostsPorts, unexpectedErrorHosts)
		usecase := NewTCPCheckUsecase(clientMock)

		// ACT
		result, err := usecase.Run(context.Background(), "localhost", 80, 1)

		// ASSERT
		if err != nil {
			t.Errorf("Unexpected error: %s", err.Error())
			return
		}

		if result.IsOk {
			t.Errorf("Unexpected status: %t", result.IsOk)
		}
	})

	t.Run("It should return error if unexpected error took place", func(t *testing.T) {
		// ARRANGE
		availableHostsPorts := []string{}
		unexpectedErrorHosts := []string{"localhost"}
		clientMock := NewTCPClientPortMock(availableHostsPorts, unexpectedErrorHosts)
		usecase := NewTCPCheckUsecase(clientMock)

		// ACT
		_, err := usecase.Run(context.Background(), "localhost", 80, 1)

		// ASSERT
		if err == nil {
			t.Errorf("Unexpected error: %s", err.Error())
		}
	})
}
