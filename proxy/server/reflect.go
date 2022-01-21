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

package server

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// A ServiceMethod represents a single gRPC service method
type ServiceMethod struct {
	serviceName       string
	methodName        string
	clientStreams     bool
	serverStreams     bool
	requestDescriptor protoreflect.MessageDescriptor
	replyDescriptor   protoreflect.MessageDescriptor
}

// FullName returns the full method name as /Package.Service/Method
func (s *ServiceMethod) FullName() string {
	return fmt.Sprintf("/%s/%s", s.serviceName, s.methodName)
}

// ClientStreams returns true if callers to this method
// send a stream of requests
func (s *ServiceMethod) ClientStreams() bool {
	return s.clientStreams
}

// ServerStreams returns true if servers that implement
// this method return a stream of responses
func (s *ServiceMethod) ServerStreams() bool {
	return s.serverStreams
}

// NewRequest returns a new a request message for this method
func (s *ServiceMethod) NewRequest() proto.Message {
	return dynamicpb.NewMessage(s.requestDescriptor)
}

// NewReply returns a new reply message for this method
func (s *ServiceMethod) NewReply() proto.Message {
	return dynamicpb.NewMessage(s.replyDescriptor)
}

// StreamDesc returns a grpc.StreamDesc used to construct
// new client streams for this method
func (s *ServiceMethod) StreamDesc() *grpc.StreamDesc {
	return &grpc.StreamDesc{
		ClientStreams: s.clientStreams,
		ServerStreams: s.serverStreams,
	}
}

// LoadServiceMap returns serviceMethod information by introspecting protocol
// buffer definitions from files registered in the supplied protoregistry.Files
// instance
func LoadServiceMap(files *protoregistry.Files) map[string]*ServiceMethod {
	out := make(map[string]*ServiceMethod)
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		sd := fd.Services()
		if sd.Len() == 0 {
			// skip files without services
			return true
		}
		for i := 0; i < sd.Len(); i++ {
			svc := sd.Get(i)
			md := svc.Methods()
			if md.Len() == 0 {
				// skip services with no methods
				continue
			}
			for j := 0; j < md.Len(); j++ {
				method := md.Get(j)
				svcMethod := &ServiceMethod{
					serviceName:       string(svc.FullName()),
					methodName:        string(method.Name()),
					clientStreams:     method.IsStreamingClient(),
					serverStreams:     method.IsStreamingServer(),
					requestDescriptor: method.Input(),
					replyDescriptor:   method.Output(),
				}
				out[svcMethod.FullName()] = svcMethod
			}
		}
		return true
	})
	return out
}

// LoadGlobalServiceMap loads service method defintions from the global
// file registry
func LoadGlobalServiceMap() map[string]*ServiceMethod {
	return LoadServiceMap(protoregistry.GlobalFiles)
}
