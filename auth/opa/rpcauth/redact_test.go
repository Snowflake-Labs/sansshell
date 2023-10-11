/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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

package rpcauth

import (
	"context"
	"testing"

	httppb "github.com/Snowflake-Labs/sansshell/services/httpoverrpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestGetRedactedInput(t *testing.T) {
	httpReq := httppb.HostHTTPRequest{
		Port:     8080,
		Hostname: "localhost",
		Protocol: "https",
		Request: &httppb.HTTPRequest{
			Method:     "POST",
			RequestUri: "/",
			Headers: []*httppb.Header{
				{Key: "key0", Values: []string{"val0"}},
			},
		},
	}
	mockInput, _ := NewRPCAuthInput(context.TODO(), "/HTTPOverRPC.HTTPOverRPC/Host", httpReq.ProtoReflect().Interface())

	for _, tc := range []struct {
		name          string
		createInputFn func() *RPCAuthInput
		assertionFn   func(RPCAuthInput)
		errFunc       func(*testing.T, error)
	}{
		{
			name: "redacted fields should be redacted",
			createInputFn: func() *RPCAuthInput {
				return mockInput
			},
			assertionFn: func(result RPCAuthInput) {
				messageType, _ := protoregistry.GlobalTypes.FindMessageByURL(mockInput.MessageType)
				resultMessage := messageType.New().Interface()
				err := protojson.Unmarshal([]byte(result.Message), resultMessage)
				assert.NoError(t, err)

				req := resultMessage.(*httppb.HostHTTPRequest)

				assert.Equal(t, "--REDACTED--", req.Request.Headers[0].Values[0]) // field with debug_redact should be redacted
				assert.Equal(t, "key0", req.Request.Headers[0].Key)               // field without debug_redact should not be redacted
			},
			errFunc: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "malformed input should return err",
			createInputFn: func() *RPCAuthInput {
				i := &RPCAuthInput{
					MessageType: "malformed",
				}
				return i
			},
			errFunc: func(t *testing.T, err error) {
				assert.NotNil(t, err)
			},
		},
		{
			name: "nil input should return nil",
			createInputFn: func() *RPCAuthInput {
				return nil
			},
			assertionFn: func(i RPCAuthInput) {
				assert.Equal(t, RPCAuthInput{}, i)
			},
			errFunc: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.createInputFn()
			result, err := getRedactedInput(input)
			if tc.assertionFn != nil {
				tc.assertionFn(result)
			}
			if tc.errFunc != nil {
				tc.errFunc(t, err)
			}
		})
	}
}
