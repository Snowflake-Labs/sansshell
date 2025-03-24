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
	"github.com/Snowflake-Labs/sansshell/auth/rpcauthz"
	"testing"

	proxypb "github.com/Snowflake-Labs/sansshell/proxy"
	"github.com/Snowflake-Labs/sansshell/proxy/testdata"
	httppb "github.com/Snowflake-Labs/sansshell/services/httpoverrpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
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
	httpReqInput, _ := rpcauthz.NewRPCAuthInput(context.TODO(), "/HTTPOverRPC.HTTPOverRPC/Host", httpReq.ProtoReflect().Interface())

	payload, _ := anypb.New(httpReq.ProtoReflect().Interface())
	proxyReq := &proxypb.ProxyRequest{
		Request: &proxypb.ProxyRequest_StreamData{
			StreamData: &proxypb.StreamData{
				StreamIds: []uint64{1},
				Payload:   payload,
			},
		},
	}
	proxyReqInput, _ := rpcauthz.NewRPCAuthInput(context.TODO(), "/Proxy.Proxy/Proxy", proxyReq.ProtoReflect().Interface())

	testReq := testdata.TestRequest{
		ListScalar: []string{"s1"},
		ListMsg: []*testdata.MyNested{
			&testdata.MyNested{
				Fine:                   "ok",
				Sensitive:              "358===",
				SensitiveBytes:         []byte("123==="),
				OneofField:             &testdata.MyNested_OneofFine{OneofFine: "oneof"},
				SensitiveRepeatedBytes: [][]byte{[]byte("123===")},
				SensitiveInt:           1,
			},
		},
		MapScalar: map[string]string{"key": "value"},
		MapMsg: map[string]*testdata.MyNested{
			"key2": &testdata.MyNested{
				Fine:           "also ok",
				Sensitive:      "456----",
				SensitiveBytes: []byte("456==="),
				OneofField:     &testdata.MyNested_OneofSensitive{OneofSensitive: "oneof"},
			},
		},
	}
	testdataInput, _ := rpcauthz.NewRPCAuthInput(context.TODO(), "/Testdata.TestService/TestUnary",
		testReq.ProtoReflect().Interface())
	for _, tc := range []struct {
		name          string
		createInputFn func() *rpcauthz.RPCAuthInput
		assertionFn   func(rpcauthz.RPCAuthInput)
		errFunc       func(*testing.T, error)
	}{
		{
			name: "redacted fields should be redacted",
			createInputFn: func() *rpcauthz.RPCAuthInput {
				return httpReqInput
			},
			assertionFn: func(result rpcauthz.RPCAuthInput) {
				messageType, _ := protoregistry.GlobalTypes.FindMessageByURL(httpReqInput.MessageType)
				resultMessage := messageType.New().Interface()
				err := protojson.Unmarshal([]byte(result.Message), resultMessage)
				assert.NoError(t, err)

				req := resultMessage.(*httppb.HostHTTPRequest)

				assert.Equal(t, "REDACTED-f47373215435fa7979debe2467a2ca7779e9cb1d11810bf7447f2f2155f13ee1", req.Request.Headers[0].Values[0]) // field with debug_redact should be redacted
				assert.Equal(t, "key0", req.Request.Headers[0].Key)                                                                            // field without debug_redact should not be redacted
			},
			errFunc: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "any containing redacted_fields should be redacted",
			createInputFn: func() *rpcauthz.RPCAuthInput {
				return proxyReqInput
			},
			assertionFn: func(result rpcauthz.RPCAuthInput) {
				messageType, _ := protoregistry.GlobalTypes.FindMessageByURL(proxyReqInput.MessageType)
				resultMessage := messageType.New().Interface()
				err := protojson.Unmarshal([]byte(result.Message), resultMessage)
				assert.NoError(t, err)

				proxyReq := resultMessage.(*proxypb.ProxyRequest)
				proxyReqPayload := proxyReq.GetStreamData().Payload
				payloadMsg, _ := proxyReqPayload.UnmarshalNew()
				httpReq := payloadMsg.(*httppb.HostHTTPRequest)

				assert.Equal(t, "REDACTED-f47373215435fa7979debe2467a2ca7779e9cb1d11810bf7447f2f2155f13ee1", httpReq.Request.Headers[0].Values[0]) // field with debug_redact should be redacted
				assert.Equal(t, "key0", httpReq.Request.Headers[0].Key)                                                                            // field without debug_redact should not be redacted
			},
			errFunc: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "redacted nested message in map or list fields",
			createInputFn: func() *rpcauthz.RPCAuthInput {
				return testdataInput
			},
			assertionFn: func(result rpcauthz.RPCAuthInput) {
				messageType, _ := protoregistry.GlobalTypes.FindMessageByURL(testdataInput.MessageType)
				resultMessage := messageType.New().Interface()
				err := protojson.Unmarshal([]byte(result.Message), resultMessage)
				assert.NoError(t, err)

				req := resultMessage.(*testdata.TestRequest)

				assert.Equal(t, "REDACTED-0c905d0153711846579c42dcd3346669ba75c0df127023b0a243d1f7390c51c4", req.ListMsg[0].Sensitive)
				assert.Equal(t, "REDACTED-4676a64752815e068c008b5068b5c7ed3ca169045cb49d98fd399aa907709afb", string(req.ListMsg[0].SensitiveBytes))
				assert.Equal(t, "REDACTED-4676a64752815e068c008b5068b5c7ed3ca169045cb49d98fd399aa907709afb", string(req.ListMsg[0].SensitiveRepeatedBytes[0]))
				assert.Equal(t, "oneof", req.ListMsg[0].GetOneofFine())
				assert.Equal(t, int64(0), req.ListMsg[0].GetSensitiveInt())
				assert.Equal(t, "REDACTED-08fe17894f2ac6df3c4530391eecc64a1cf84593f85f1f018d0aae7581d28d4e", req.MapMsg["key2"].Sensitive)
				assert.Equal(t, "REDACTED-fd1a71e8a6933fdaa5cfe7944c8d7533a79288c0c99b32f12753991bdee5b906", string(req.MapMsg["key2"].SensitiveBytes))
				assert.Equal(t, "REDACTED-d24dd488138ce5c370d9db173fc97fa876ac17f11ded4eeaeb4d6170afd22b2d", req.MapMsg["key2"].GetOneofSensitive())
			},
			errFunc: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "malformed input should return err",
			createInputFn: func() *rpcauthz.RPCAuthInput {
				i := &rpcauthz.RPCAuthInput{
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
			createInputFn: func() *rpcauthz.RPCAuthInput {
				return nil
			},
			assertionFn: func(i rpcauthz.RPCAuthInput) {
				assert.Equal(t, rpcauthz.RPCAuthInput{}, i)
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
