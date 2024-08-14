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
	t.Run("It should return ok if server reply ok", func(t *testing.T) {
		// ARRANGE
		targetHosts := []string{"target:3000"}
		availableHostsPorts := []string{"localhost:80"}
		unexpectedErrorHosts := []string{}
		clientMock := NewNetworkClientProxyMock(targetHosts, availableHostsPorts, unexpectedErrorHosts)
		usecase := NewTCPCheckUseCase(clientMock)

		// ACT
		c, err := usecase.Run(context.Background(), "localhost", 80, 1)

		// ASSERT
		if err != nil {
			t.Errorf("Unexpected error: %s", err.Error())
		}

		for resp := range c {
			if resp.Error != nil {
				t.Errorf("Unexpected response %d error: %s", resp.Index, err.Error())
			}

			if !resp.Resp.Ok {
				t.Errorf("Unexpected response %d status: %t", resp.Index, resp.Resp.Ok)
			}
		}
	})

	t.Run("It should return NOT ok if server reply with not ok status", func(t *testing.T) {
		// ARRANGE
		targetHosts := []string{"target:3000"}
		availableHostsPorts := []string{}
		unexpectedErrorHosts := []string{}
		clientMock := NewNetworkClientProxyMock(targetHosts, availableHostsPorts, unexpectedErrorHosts)
		usecase := NewTCPCheckUseCase(clientMock)

		// ACT
		c, err := usecase.Run(context.Background(), "localhost", 80, 1)

		// ASSERT
		if err != nil {
			t.Errorf("Unexpected error: %s", err.Error())
		}

		for resp := range c {
			if resp.Error != nil {
				t.Errorf("Unexpected response %d error: %s", resp.Index, err.Error())
			}

			if resp.Resp.Ok {
				t.Errorf("Unexpected response %d status: %t", resp.Index, resp.Resp.Ok)
			}
		}
	})

	t.Run("It should return error if server reply with unexpected error", func(t *testing.T) {
		// ARRANGE
		targetHosts := []string{"target:3000"}
		availableHostsPorts := []string{}
		unexpectedErrorHosts := []string{"localhost"}
		clientMock := NewNetworkClientProxyMock(targetHosts, availableHostsPorts, unexpectedErrorHosts)
		usecase := NewTCPCheckUseCase(clientMock)

		// ACT
		_, err := usecase.Run(context.Background(), "localhost", 80, 1)

		// ASSERT
		if err == nil {
			t.Errorf("Error unexpectedly is nil")
			return
		}
	})
}
