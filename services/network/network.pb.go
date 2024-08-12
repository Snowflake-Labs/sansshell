// Copyright (c) 2022 Snowflake Inc. All rights reserved.
//
//Licensed under the Apache License, Version 2.0 (the
//"License"); you may not use this file except in compliance
//with the License.  You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing,
//software distributed under the License is distributed on an
//"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
//KIND, either express or implied.  See the License for the
//specific language governing permissions and limitations
//under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.3
// source: network.proto

package network

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// TCP Check describes the request to make check of tcp connectivity check
type TCPCheckRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The hostname to check
	Hostname string `protobuf:"bytes,1,opt,name=hostname,proto3" json:"hostname,omitempty"`
	// The port to check
	Port uint32 `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
	// The timeout in seconds
	Timeout uint32 `protobuf:"varint,3,opt,name=timeout,proto3" json:"timeout,omitempty"`
}

func (x *TCPCheckRequest) Reset() {
	*x = TCPCheckRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_network_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TCPCheckRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TCPCheckRequest) ProtoMessage() {}

func (x *TCPCheckRequest) ProtoReflect() protoreflect.Message {
	mi := &file_network_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TCPCheckRequest.ProtoReflect.Descriptor instead.
func (*TCPCheckRequest) Descriptor() ([]byte, []int) {
	return file_network_proto_rawDescGZIP(), []int{0}
}

func (x *TCPCheckRequest) GetHostname() string {
	if x != nil {
		return x.Hostname
	}
	return ""
}

func (x *TCPCheckRequest) GetPort() uint32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *TCPCheckRequest) GetTimeout() uint32 {
	if x != nil {
		return x.Timeout
	}
	return 0
}

// TCPCheckReply describes the result of a rpc
type TCPCheckReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ok         bool    `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
	FailReason *string `protobuf:"bytes,2,opt,name=failReason,proto3,oneof" json:"failReason,omitempty"`
}

func (x *TCPCheckReply) Reset() {
	*x = TCPCheckReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_network_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TCPCheckReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TCPCheckReply) ProtoMessage() {}

func (x *TCPCheckReply) ProtoReflect() protoreflect.Message {
	mi := &file_network_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TCPCheckReply.ProtoReflect.Descriptor instead.
func (*TCPCheckReply) Descriptor() ([]byte, []int) {
	return file_network_proto_rawDescGZIP(), []int{1}
}

func (x *TCPCheckReply) GetOk() bool {
	if x != nil {
		return x.Ok
	}
	return false
}

func (x *TCPCheckReply) GetFailReason() string {
	if x != nil && x.FailReason != nil {
		return *x.FailReason
	}
	return ""
}

var File_network_proto protoreflect.FileDescriptor

var file_network_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x22, 0x5b, 0x0a, 0x0f, 0x54, 0x43, 0x50, 0x43,
	0x68, 0x65, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x68,
	0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x68,
	0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x74,
	0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x07, 0x74, 0x69,
	0x6d, 0x65, 0x6f, 0x75, 0x74, 0x22, 0x53, 0x0a, 0x0d, 0x54, 0x43, 0x50, 0x43, 0x68, 0x65, 0x63,
	0x6b, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x02, 0x6f, 0x6b, 0x12, 0x23, 0x0a, 0x0a, 0x66, 0x61, 0x69, 0x6c, 0x52, 0x65,
	0x61, 0x73, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x0a, 0x66, 0x61,
	0x69, 0x6c, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x88, 0x01, 0x01, 0x42, 0x0d, 0x0a, 0x0b, 0x5f,
	0x66, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x32, 0x49, 0x0a, 0x07, 0x4e, 0x65,
	0x74, 0x77, 0x6f, 0x72, 0x6b, 0x12, 0x3e, 0x0a, 0x08, 0x54, 0x43, 0x50, 0x43, 0x68, 0x65, 0x63,
	0x6b, 0x12, 0x18, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x54, 0x43, 0x50, 0x43,
	0x68, 0x65, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x4e, 0x65,
	0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x54, 0x43, 0x50, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x52, 0x65,
	0x70, 0x6c, 0x79, 0x22, 0x00, 0x42, 0x2d, 0x5a, 0x2b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x6e, 0x6f, 0x77, 0x66, 0x6c, 0x61, 0x6b, 0x65, 0x2d, 0x4c, 0x61,
	0x62, 0x73, 0x2f, 0x73, 0x61, 0x6e, 0x73, 0x73, 0x68, 0x65, 0x6c, 0x6c, 0x2f, 0x6e, 0x65, 0x74,
	0x77, 0x6f, 0x72, 0x6b, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_network_proto_rawDescOnce sync.Once
	file_network_proto_rawDescData = file_network_proto_rawDesc
)

func file_network_proto_rawDescGZIP() []byte {
	file_network_proto_rawDescOnce.Do(func() {
		file_network_proto_rawDescData = protoimpl.X.CompressGZIP(file_network_proto_rawDescData)
	})
	return file_network_proto_rawDescData
}

var file_network_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_network_proto_goTypes = []any{
	(*TCPCheckRequest)(nil), // 0: Network.TCPCheckRequest
	(*TCPCheckReply)(nil),   // 1: Network.TCPCheckReply
}
var file_network_proto_depIdxs = []int32{
	0, // 0: Network.Network.TCPCheck:input_type -> Network.TCPCheckRequest
	1, // 1: Network.Network.TCPCheck:output_type -> Network.TCPCheckReply
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_network_proto_init() }
func file_network_proto_init() {
	if File_network_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_network_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*TCPCheckRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_network_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*TCPCheckReply); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_network_proto_msgTypes[1].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_network_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_network_proto_goTypes,
		DependencyIndexes: file_network_proto_depIdxs,
		MessageInfos:      file_network_proto_msgTypes,
	}.Build()
	File_network_proto = out.File
	file_network_proto_rawDesc = nil
	file_network_proto_goTypes = nil
	file_network_proto_depIdxs = nil
}
