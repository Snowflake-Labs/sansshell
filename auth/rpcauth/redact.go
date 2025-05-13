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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
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

func redactListField(value protoreflect.Value) error {
	for i := 0; i < value.List().Len(); i++ {
		errRedact := redactFields(value.List().Get(i).Message())
		if errRedact != nil {
			return errRedact
		}
	}
	return nil
}

func redactMapField(value protoreflect.Value) error {
	var err error
	value.Map().Range(func(mapKey protoreflect.MapKey, mapValue protoreflect.Value) bool {
		errRedact := redactFields(mapValue.Message())
		if errRedact != nil {
			err = errRedact
			return false
		}
		return true
	})
	return err
}

var anypbFullName = (&anypb.Any{}).ProtoReflect().Descriptor().FullName()

func redactAny(message protoreflect.Message, descriptor protoreflect.FieldDescriptor, value protoreflect.Value) error {
	anyMsg, ok := value.Message().Interface().(*anypb.Any) // cast value to redactAny
	if !ok {
		return fmt.Errorf("failed to cast message into any")
	}
	originalMsg, errUnmarshal := anyMsg.UnmarshalNew() // unmarshal any into original message
	if errUnmarshal != nil {
		return fmt.Errorf("failed to unmarshal anypb: %v", errUnmarshal)
	}
	errRedact := redactFields(originalMsg.ProtoReflect()) // redact original message
	if errRedact != nil {
		return errRedact
	}
	redactedAny, errAny := anypb.New(originalMsg) // cast redacted message back to any
	if errAny != nil {
		return fmt.Errorf("failed to cast into anypb: %v", errAny)
	}
	message.Set(descriptor, protoreflect.ValueOf(redactedAny.ProtoReflect())) // set the redacted

	return nil
}

func redactNestedField(message protoreflect.Message, descriptor protoreflect.FieldDescriptor, value protoreflect.Value) error {
	switch {
	case descriptor.IsList() && isMessage(descriptor):
		return redactListField(value)
	case descriptor.IsMap() && isMessage(descriptor.MapValue()):
		// Only when map value are of Message type, we recurse.
		return redactMapField(value)
	case descriptor.Message() != nil && descriptor.Message().FullName() == anypbFullName:
		return redactAny(message, descriptor, value)
	case !descriptor.IsMap() && isMessage(descriptor):
		return redactFields(value.Message())
	}
	return nil
}

func redactSingleField(message protoreflect.Message, descriptor protoreflect.FieldDescriptor) {
	// We redact the field by hashing the value and replacing it with the hash whenever
	// possible. If we don't know how to do so, we clear the field.
	// Add more cases as needed.
	if descriptor.Kind() == protoreflect.StringKind {
		if descriptor.Cardinality() != protoreflect.Repeated {
			if val := message.Get(descriptor).String(); val != "" {
				message.Set(descriptor, protoreflect.ValueOfString(fmt.Sprintf("REDACTED-%x", sha256.Sum256([]byte(val)))))
			}
		} else {
			list := message.Mutable(descriptor).List()
			for i := 0; i < list.Len(); i++ {
				if val := list.Get(i).String(); val != "" {
					list.Set(i, protoreflect.ValueOfString(fmt.Sprintf("REDACTED-%x", sha256.Sum256([]byte(val)))))
				}
			}
		}
	} else if descriptor.Kind() == protoreflect.BytesKind {
		if descriptor.Cardinality() != protoreflect.Repeated {
			if val := message.Get(descriptor).Bytes(); val != nil {
				message.Set(descriptor, protoreflect.ValueOfBytes([]byte(fmt.Sprintf("REDACTED-%x", sha256.Sum256(val)))))
			}
		} else {
			list := message.Mutable(descriptor).List()
			for i := 0; i < list.Len(); i++ {
				if val := list.Get(i).Bytes(); val != nil {
					list.Set(i, protoreflect.ValueOfBytes([]byte(fmt.Sprintf("REDACTED-%x", sha256.Sum256(val)))))
				}
			}
		}
	} else {
		// other than string or bytes, clear it
		message.Clear(descriptor)
	}
}

func redactFields(message protoreflect.Message) error {
	var err error
	message.Range(
		func(descriptor protoreflect.FieldDescriptor, value protoreflect.Value) bool {
			if isDebugRedactEnabled(descriptor) {
				redactSingleField(message, descriptor)
				return true
			}
			errNested := redactNestedField(message, descriptor, value)
			if errNested != nil {
				err = errNested
				return false
			}
			return true
		},
	)
	return err
}

func getRedactedInput(input *RPCAuthInput) (RPCAuthInput, error) {
	if input == nil {
		return RPCAuthInput{}, nil
	}
	redactedInput := RPCAuthInput{
		Method:      input.Method,
		MessageType: input.MessageType,
		Peer:        input.Peer,
		Host:        input.Host,
		Environment: input.Environment,
		Extensions:  input.Extensions,
	}

	if input.Metadata != nil {
		redactedInput.Metadata = metadata.MD{}

		for k, v := range input.Metadata {
			if isRedactedMetadata(k) {
				continue
			}

			redactedInput.Metadata[k] = v
		}
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
		errRedact := redactFields(redactedMessage.ProtoReflect())
		if errRedact != nil {
			return RPCAuthInput{}, fmt.Errorf("failed to redact message fields: %v", errRedact)
		}
	}
	marshaled, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(redactedMessage)
	if err != nil {
		return RPCAuthInput{}, status.Errorf(codes.Internal, "error marshalling request for auth: %v", err)
	}
	redactedInput.Message = json.RawMessage(marshaled)
	return redactedInput, nil
}

func isRedactedMetadata(key string) bool {
	if strings.ToLower(key) == "authorization" {
		return true
	}

	return false
}
