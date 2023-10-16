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
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"

	proxypb "github.com/Snowflake-Labs/sansshell/proxy"
)

func isMessage(descriptor protoreflect.FieldDescriptor) bool {
	return descriptor.Kind() == protoreflect.MessageKind || descriptor.Kind() == protoreflect.GroupKind
}

func isDebugRedactEnabled(fd protoreflect.FieldDescriptor) bool {
	opts, ok := fd.Options().(*descriptorpb.FieldOptions)
	if !ok {
		return false
	}
	return opts.GetDebugRedact()
}

func redactListField(value protoreflect.Value) {
	for i := 0; i < value.List().Len(); i++ {
		redactFields(value.List().Get(i).Message())
	}
}

func redactMapField(value protoreflect.Value) {
	value.Map().Range(func(mapKey protoreflect.MapKey, mapValue protoreflect.Value) bool {
		redactFields(mapValue.Message())
		return true
	})
}

func redactNestedMessage(message protoreflect.Message, descriptor protoreflect.FieldDescriptor, value protoreflect.Value) {
	switch {
	case descriptor.IsList() && isMessage(descriptor):
		redactListField(value)
	case descriptor.IsMap() && isMessage(descriptor):
		redactMapField(value)
	case !descriptor.IsMap() && isMessage(descriptor):
		redactFields(value.Message())
	}
}

func redactSingleField(message protoreflect.Message, descriptor protoreflect.FieldDescriptor) {
	if descriptor.Kind() == protoreflect.StringKind {
		if descriptor.Cardinality() != protoreflect.Repeated {
			message.Set(descriptor, protoreflect.ValueOfString("--REDACTED--"))
		} else {
			list := message.Mutable(descriptor).List()
			for i := 0; i < list.Len(); i++ {
				list.Set(i, protoreflect.ValueOfString("--REDACTED--"))
			}
		}
	} else {
		// other than string, clear it
		message.Clear(descriptor)
	}
}

func redactFields(message protoreflect.Message) {
	message.Range(
		func(descriptor protoreflect.FieldDescriptor, value protoreflect.Value) bool {
			if isDebugRedactEnabled(descriptor) {
				redactSingleField(message, descriptor)
				return true
			}
			redactNestedMessage(message, descriptor, value)
			return true
		},
	)
}

func getRedactedInput(input *RPCAuthInput) (RPCAuthInput, error) {
	if input == nil {
		return RPCAuthInput{}, nil
	}
	redactedInput := RPCAuthInput{
		Method:      input.Method,
		MessageType: input.MessageType,
		Metadata:    input.Metadata,
		Peer:        input.Peer,
		Host:        input.Host,
		Environment: input.Environment,
		Extensions:  input.Extensions,
	}
	if input.MessageType == "" {
		return redactedInput, nil
	}
	var redactedMessage protoreflect.ProtoMessage
	if input != nil {
		// Transform the rpcauth input into the original proto
		messageType, err := protoregistry.GlobalTypes.FindMessageByURL(input.MessageType)
		if err != nil {
			return RPCAuthInput{}, fmt.Errorf("unable to find proto type %v: %v", input.MessageType, err)
		}
		redactedMessage = messageType.New().Interface()
		if err := protojson.Unmarshal([]byte(input.Message), redactedMessage); err != nil {
			return RPCAuthInput{}, fmt.Errorf("could not marshal input into %v: %v", input.MessageType, err)
		}
		if proxyReq, ok := redactedMessage.(*proxypb.ProxyRequest); ok && proxyReq.GetStreamData() != nil {
			// redact the payload of proxypb.StreamData
			streamData := proxyReq.GetStreamData()
			payload, err := streamData.Payload.UnmarshalNew() // unmarshal the payload contents for redaction
			if err != nil {
				return RPCAuthInput{}, fmt.Errorf("failed to unmarshal stream data: %v", err)
			}
			redactFields(payload.ProtoReflect()) // redact the payload
			any, err := anypb.New(payload)       // cast it into anypb
			if err != nil {
				return RPCAuthInput{}, fmt.Errorf("failed to create new anypb while redacting: %v", err)
			}
			streamData.Payload = any // set redactedMessage streamData's payload to the redacted one
		} else {
			redactFields(redactedMessage.ProtoReflect())
		}
	}
	marshaled, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(redactedMessage)
	if err != nil {
		return RPCAuthInput{}, status.Errorf(codes.Internal, "error marshalling request for auth: %v", err)
	}
	redactedInput.Message = json.RawMessage(marshaled)
	return redactedInput, nil
}
