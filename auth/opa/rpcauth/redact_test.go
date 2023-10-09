package rpcauth

import (
	"context"
	"testing"

	httpPB "github.com/Snowflake-Labs/sansshell/services/httpoverrpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestGetRedactedInput(t *testing.T) {
	httpReq := httpPB.HostHTTPRequest{
		Port:     8080,
		Hostname: "localhost",
		Protocol: "https",
		Request: &httpPB.HTTPRequest{
			Method:     "POST",
			RequestUri: "/",
			Headers: []*httpPB.Header{
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
				protojson.Unmarshal([]byte(result.Message), resultMessage)

				req := resultMessage.(*httpPB.HostHTTPRequest)

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
