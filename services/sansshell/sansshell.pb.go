// Copyright (c) 2019 Snowflake Inc. All rights reserved.
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
// 	protoc-gen-go v1.27.1
// 	protoc        v3.21.5
// source: sansshell.proto

package sansshell

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type SetVerbosityRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Level int32 `protobuf:"varint,1,opt,name=Level,proto3" json:"Level,omitempty"`
}

func (x *SetVerbosityRequest) Reset() {
	*x = SetVerbosityRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sansshell_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetVerbosityRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetVerbosityRequest) ProtoMessage() {}

func (x *SetVerbosityRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sansshell_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetVerbosityRequest.ProtoReflect.Descriptor instead.
func (*SetVerbosityRequest) Descriptor() ([]byte, []int) {
	return file_sansshell_proto_rawDescGZIP(), []int{0}
}

func (x *SetVerbosityRequest) GetLevel() int32 {
	if x != nil {
		return x.Level
	}
	return 0
}

type VerbosityReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Level int32 `protobuf:"varint,1,opt,name=Level,proto3" json:"Level,omitempty"`
}

func (x *VerbosityReply) Reset() {
	*x = VerbosityReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sansshell_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VerbosityReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VerbosityReply) ProtoMessage() {}

func (x *VerbosityReply) ProtoReflect() protoreflect.Message {
	mi := &file_sansshell_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VerbosityReply.ProtoReflect.Descriptor instead.
func (*VerbosityReply) Descriptor() ([]byte, []int) {
	return file_sansshell_proto_rawDescGZIP(), []int{1}
}

func (x *VerbosityReply) GetLevel() int32 {
	if x != nil {
		return x.Level
	}
	return 0
}

type VersionResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Version string `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
}

func (x *VersionResponse) Reset() {
	*x = VersionResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sansshell_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VersionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VersionResponse) ProtoMessage() {}

func (x *VersionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_sansshell_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VersionResponse.ProtoReflect.Descriptor instead.
func (*VersionResponse) Descriptor() ([]byte, []int) {
	return file_sansshell_proto_rawDescGZIP(), []int{2}
}

func (x *VersionResponse) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

var File_sansshell_proto protoreflect.FileDescriptor

var file_sansshell_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x73, 0x61, 0x6e, 0x73, 0x73, 0x68, 0x65, 0x6c, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x09, 0x53, 0x61, 0x6e, 0x73, 0x73, 0x68, 0x65, 0x6c, 0x6c, 0x1a, 0x1b, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d,
	0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x2b, 0x0a, 0x13, 0x53, 0x65, 0x74,
	0x56, 0x65, 0x72, 0x62, 0x6f, 0x73, 0x69, 0x74, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x14, 0x0a, 0x05, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x05, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x22, 0x26, 0x0a, 0x0e, 0x56, 0x65, 0x72, 0x62, 0x6f, 0x73,
	0x69, 0x74, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x4c, 0x65, 0x76, 0x65,
	0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x22, 0x2b,
	0x0a, 0x0f, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x32, 0x9b, 0x01, 0x0a, 0x07,
	0x4c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x12, 0x4b, 0x0a, 0x0c, 0x53, 0x65, 0x74, 0x56, 0x65,
	0x72, 0x62, 0x6f, 0x73, 0x69, 0x74, 0x79, 0x12, 0x1e, 0x2e, 0x53, 0x61, 0x6e, 0x73, 0x73, 0x68,
	0x65, 0x6c, 0x6c, 0x2e, 0x53, 0x65, 0x74, 0x56, 0x65, 0x72, 0x62, 0x6f, 0x73, 0x69, 0x74, 0x79,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x19, 0x2e, 0x53, 0x61, 0x6e, 0x73, 0x73, 0x68,
	0x65, 0x6c, 0x6c, 0x2e, 0x56, 0x65, 0x72, 0x62, 0x6f, 0x73, 0x69, 0x74, 0x79, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x22, 0x00, 0x12, 0x43, 0x0a, 0x0c, 0x47, 0x65, 0x74, 0x56, 0x65, 0x72, 0x62, 0x6f,
	0x73, 0x69, 0x74, 0x79, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x19, 0x2e, 0x53,
	0x61, 0x6e, 0x73, 0x73, 0x68, 0x65, 0x6c, 0x6c, 0x2e, 0x56, 0x65, 0x72, 0x62, 0x6f, 0x73, 0x69,
	0x74, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x32, 0x48, 0x0a, 0x05, 0x53, 0x74, 0x61,
	0x74, 0x65, 0x12, 0x3f, 0x0a, 0x07, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x16, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x1a, 0x2e, 0x53, 0x61, 0x6e, 0x73, 0x73, 0x68, 0x65, 0x6c,
	0x6c, 0x2e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x42, 0x2f, 0x5a, 0x2d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x53, 0x6e, 0x6f, 0x77, 0x66, 0x6c, 0x61, 0x6b, 0x65, 0x2d, 0x4c, 0x61, 0x62, 0x73,
	0x2f, 0x73, 0x61, 0x6e, 0x73, 0x73, 0x68, 0x65, 0x6c, 0x6c, 0x2f, 0x73, 0x61, 0x6e, 0x73, 0x73,
	0x68, 0x65, 0x6c, 0x6c, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sansshell_proto_rawDescOnce sync.Once
	file_sansshell_proto_rawDescData = file_sansshell_proto_rawDesc
)

func file_sansshell_proto_rawDescGZIP() []byte {
	file_sansshell_proto_rawDescOnce.Do(func() {
		file_sansshell_proto_rawDescData = protoimpl.X.CompressGZIP(file_sansshell_proto_rawDescData)
	})
	return file_sansshell_proto_rawDescData
}

var file_sansshell_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_sansshell_proto_goTypes = []interface{}{
	(*SetVerbosityRequest)(nil), // 0: Sansshell.SetVerbosityRequest
	(*VerbosityReply)(nil),      // 1: Sansshell.VerbosityReply
	(*VersionResponse)(nil),     // 2: Sansshell.VersionResponse
	(*emptypb.Empty)(nil),       // 3: google.protobuf.Empty
}
var file_sansshell_proto_depIdxs = []int32{
	0, // 0: Sansshell.Logging.SetVerbosity:input_type -> Sansshell.SetVerbosityRequest
	3, // 1: Sansshell.Logging.GetVerbosity:input_type -> google.protobuf.Empty
	3, // 2: Sansshell.State.Version:input_type -> google.protobuf.Empty
	1, // 3: Sansshell.Logging.SetVerbosity:output_type -> Sansshell.VerbosityReply
	1, // 4: Sansshell.Logging.GetVerbosity:output_type -> Sansshell.VerbosityReply
	2, // 5: Sansshell.State.Version:output_type -> Sansshell.VersionResponse
	3, // [3:6] is the sub-list for method output_type
	0, // [0:3] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_sansshell_proto_init() }
func file_sansshell_proto_init() {
	if File_sansshell_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sansshell_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetVerbosityRequest); i {
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
		file_sansshell_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VerbosityReply); i {
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
		file_sansshell_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VersionResponse); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_sansshell_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_sansshell_proto_goTypes,
		DependencyIndexes: file_sansshell_proto_depIdxs,
		MessageInfos:      file_sansshell_proto_msgTypes,
	}.Build()
	File_sansshell_proto = out.File
	file_sansshell_proto_rawDesc = nil
	file_sansshell_proto_goTypes = nil
	file_sansshell_proto_depIdxs = nil
}
