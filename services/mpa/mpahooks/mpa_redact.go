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

package mpahooks

import (
	"fmt"

	"github.com/Snowflake-Labs/sansshell/services/mpa/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
)

// RedactFieldsForMPA processes an Any proto message and redacts (sets to nil/zero)
// any fields that are marked with the mpa_redacted annotation.
// Returns true if the message was modified.
func RedactFieldsForMPA(anyMsg *anypb.Any) (bool, error) {
	// First extract the message from Any
	msg, err := anyMsg.UnmarshalNew()
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal Any message: %v", err)
	}

	// Process the message to redact marked fields
	modified := redactMessageFields(msg)

	// If we made changes, update the Any message
	if modified {
		if err := anyMsg.MarshalFrom(msg); err != nil {
			return false, fmt.Errorf("failed to re-marshal message: %v", err)
		}
	}

	return modified, nil
}

// redactMessageFields recursively processes a message and redacts fields
// marked with the mpa_redacted annotation.
// Returns true if any field was redacted.
func redactMessageFields(message proto.Message) bool {
	modified := false

	// Get the reflective view of the message
	m := message.ProtoReflect()

	// Iterate through all fields in the message
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		// Check if this field has the mpa_redacted option set
		opts := fd.Options().(*descriptorpb.FieldOptions)

		if proto.GetExtension(opts, annotations.E_MpaRedacted).(bool) {
			m.Clear(fd)
			modified = true
			return true // Continue iteration
		}

		// If it's a message field that's not nil, recursively process it
		if fd.Kind() == protoreflect.MessageKind && v.IsValid() {
			if fd.IsList() {
				// Handle repeated message fields
				list := v.List()
				for i := 0; i < list.Len(); i++ {
					item := list.Get(i)
					if item.Message().IsValid() {
						// Create a new proto.Message from this item
						nestedMsg := item.Message().Interface()
						if redactMessageFields(nestedMsg) {
							modified = true
						}
					}
				}
			} else if fd.IsMap() {
				// Handle map fields where values are messages
				if fd.MapValue().Kind() == protoreflect.MessageKind {
					mapVal := v.Map()
					mapVal.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
						if v.Message().IsValid() {
							nestedMsg := v.Message().Interface()
							if redactMessageFields(nestedMsg) {
								modified = true
							}
						}
						return true
					})
				}
			} else if v.Message().IsValid() {
				// Handle regular message fields
				nestedMsg := v.Message().Interface()
				if redactMessageFields(nestedMsg) {
					modified = true
				}
			}
		}

		return true // Continue iteration
	})

	return modified
}
