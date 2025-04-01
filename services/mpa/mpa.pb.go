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
// 	protoc        v5.28.3
// source: mpa.proto

package mpa

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Action struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The user that created the request.
	User string `protobuf:"bytes,1,opt,name=user,proto3" json:"user,omitempty"`
	// User-supplied information on why the request is being made.
	Justification string `protobuf:"bytes,2,opt,name=justification,proto3" json:"justification,omitempty"`
	// The GRPC method name, as '/Package.Service/Method'
	Method string `protobuf:"bytes,3,opt,name=method,proto3" json:"method,omitempty"`
	// The request protocol buffer.
	Message *anypb.Any `protobuf:"bytes,4,opt,name=message,proto3" json:"message,omitempty"`
	// Method-wide MPA requested, will ignore `message` and only match request on
	// `method`.
	MethodWideMpa bool `protobuf:"varint,5,opt,name=method_wide_mpa,json=methodWideMpa,proto3" json:"method_wide_mpa,omitempty"`
}

func (x *Action) Reset() {
	*x = Action{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Action) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Action) ProtoMessage() {}

func (x *Action) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Action.ProtoReflect.Descriptor instead.
func (*Action) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{0}
}

func (x *Action) GetUser() string {
	if x != nil {
		return x.User
	}
	return ""
}

func (x *Action) GetJustification() string {
	if x != nil {
		return x.Justification
	}
	return ""
}

func (x *Action) GetMethod() string {
	if x != nil {
		return x.Method
	}
	return ""
}

func (x *Action) GetMessage() *anypb.Any {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *Action) GetMethodWideMpa() bool {
	if x != nil {
		return x.MethodWideMpa
	}
	return false
}

type Principal struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The principal identifier (e.g. a username or service role)
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Auxiliary groups associated with this principal.
	Groups []string `protobuf:"bytes,2,rep,name=groups,proto3" json:"groups,omitempty"`
}

func (x *Principal) Reset() {
	*x = Principal{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Principal) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Principal) ProtoMessage() {}

func (x *Principal) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Principal.ProtoReflect.Descriptor instead.
func (*Principal) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{1}
}

func (x *Principal) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Principal) GetGroups() []string {
	if x != nil {
		return x.Groups
	}
	return nil
}

type StoreRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The GRPC method name, as '/Package.Service/Method'
	Method string `protobuf:"bytes,1,opt,name=method,proto3" json:"method,omitempty"`
	// The request protocol buffer.
	Message *anypb.Any `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	// Method-wide MPA requested, will ignore `message` and only match request on
	// `method`.
	MethodWideMpa bool `protobuf:"varint,5,opt,name=method_wide_mpa,json=methodWideMpa,proto3" json:"method_wide_mpa,omitempty"`
}

func (x *StoreRequest) Reset() {
	*x = StoreRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StoreRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StoreRequest) ProtoMessage() {}

func (x *StoreRequest) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StoreRequest.ProtoReflect.Descriptor instead.
func (*StoreRequest) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{2}
}

func (x *StoreRequest) GetMethod() string {
	if x != nil {
		return x.Method
	}
	return ""
}

func (x *StoreRequest) GetMessage() *anypb.Any {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *StoreRequest) GetMethodWideMpa() bool {
	if x != nil {
		return x.MethodWideMpa
	}
	return false
}

type StoreResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id     string  `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Action *Action `protobuf:"bytes,2,opt,name=action,proto3" json:"action,omitempty"`
	// All approvers of the request. Storing is idempotent, so
	// approvers may be non-empty if we're storing a previously
	// approved command.
	Approver []*Principal `protobuf:"bytes,3,rep,name=approver,proto3" json:"approver,omitempty"`
}

func (x *StoreResponse) Reset() {
	*x = StoreResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StoreResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StoreResponse) ProtoMessage() {}

func (x *StoreResponse) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StoreResponse.ProtoReflect.Descriptor instead.
func (*StoreResponse) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{3}
}

func (x *StoreResponse) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *StoreResponse) GetAction() *Action {
	if x != nil {
		return x.Action
	}
	return nil
}

func (x *StoreResponse) GetApprover() []*Principal {
	if x != nil {
		return x.Approver
	}
	return nil
}

type ApproveRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Approve takes an action instead of an ID to improve auditability
	// and allow richer authorization logic.
	Action *Action `protobuf:"bytes,1,opt,name=action,proto3" json:"action,omitempty"`
}

func (x *ApproveRequest) Reset() {
	*x = ApproveRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApproveRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApproveRequest) ProtoMessage() {}

func (x *ApproveRequest) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApproveRequest.ProtoReflect.Descriptor instead.
func (*ApproveRequest) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{4}
}

func (x *ApproveRequest) GetAction() *Action {
	if x != nil {
		return x.Action
	}
	return nil
}

type ApproveResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ApproveResponse) Reset() {
	*x = ApproveResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApproveResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApproveResponse) ProtoMessage() {}

func (x *ApproveResponse) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApproveResponse.ProtoReflect.Descriptor instead.
func (*ApproveResponse) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{5}
}

type WaitForApprovalRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *WaitForApprovalRequest) Reset() {
	*x = WaitForApprovalRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WaitForApprovalRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WaitForApprovalRequest) ProtoMessage() {}

func (x *WaitForApprovalRequest) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WaitForApprovalRequest.ProtoReflect.Descriptor instead.
func (*WaitForApprovalRequest) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{6}
}

func (x *WaitForApprovalRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type WaitForApprovalResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *WaitForApprovalResponse) Reset() {
	*x = WaitForApprovalResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WaitForApprovalResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WaitForApprovalResponse) ProtoMessage() {}

func (x *WaitForApprovalResponse) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WaitForApprovalResponse.ProtoReflect.Descriptor instead.
func (*WaitForApprovalResponse) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{7}
}

type ListRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ListRequest) Reset() {
	*x = ListRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRequest) ProtoMessage() {}

func (x *ListRequest) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRequest.ProtoReflect.Descriptor instead.
func (*ListRequest) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{8}
}

type ListResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Item []*ListResponse_Item `protobuf:"bytes,1,rep,name=item,proto3" json:"item,omitempty"`
}

func (x *ListResponse) Reset() {
	*x = ListResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListResponse) ProtoMessage() {}

func (x *ListResponse) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListResponse.ProtoReflect.Descriptor instead.
func (*ListResponse) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{9}
}

func (x *ListResponse) GetItem() []*ListResponse_Item {
	if x != nil {
		return x.Item
	}
	return nil
}

type GetRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *GetRequest) Reset() {
	*x = GetRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRequest) ProtoMessage() {}

func (x *GetRequest) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRequest.ProtoReflect.Descriptor instead.
func (*GetRequest) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{10}
}

func (x *GetRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type GetResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Action *Action `protobuf:"bytes,1,opt,name=action,proto3" json:"action,omitempty"`
	// All approvers of the request.
	Approver []*Principal `protobuf:"bytes,2,rep,name=approver,proto3" json:"approver,omitempty"`
}

func (x *GetResponse) Reset() {
	*x = GetResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetResponse) ProtoMessage() {}

func (x *GetResponse) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetResponse.ProtoReflect.Descriptor instead.
func (*GetResponse) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{11}
}

func (x *GetResponse) GetAction() *Action {
	if x != nil {
		return x.Action
	}
	return nil
}

func (x *GetResponse) GetApprover() []*Principal {
	if x != nil {
		return x.Approver
	}
	return nil
}

type ClearRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Action *Action `protobuf:"bytes,1,opt,name=action,proto3" json:"action,omitempty"`
}

func (x *ClearRequest) Reset() {
	*x = ClearRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[12]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClearRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClearRequest) ProtoMessage() {}

func (x *ClearRequest) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[12]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClearRequest.ProtoReflect.Descriptor instead.
func (*ClearRequest) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{12}
}

func (x *ClearRequest) GetAction() *Action {
	if x != nil {
		return x.Action
	}
	return nil
}

type ClearResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ClearResponse) Reset() {
	*x = ClearResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[13]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClearResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClearResponse) ProtoMessage() {}

func (x *ClearResponse) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[13]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClearResponse.ProtoReflect.Descriptor instead.
func (*ClearResponse) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{13}
}

type ListResponse_Item struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Action   *Action      `protobuf:"bytes,1,opt,name=action,proto3" json:"action,omitempty"`
	Approver []*Principal `protobuf:"bytes,2,rep,name=approver,proto3" json:"approver,omitempty"`
	Id       string       `protobuf:"bytes,3,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *ListResponse_Item) Reset() {
	*x = ListResponse_Item{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mpa_proto_msgTypes[14]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListResponse_Item) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListResponse_Item) ProtoMessage() {}

func (x *ListResponse_Item) ProtoReflect() protoreflect.Message {
	mi := &file_mpa_proto_msgTypes[14]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListResponse_Item.ProtoReflect.Descriptor instead.
func (*ListResponse_Item) Descriptor() ([]byte, []int) {
	return file_mpa_proto_rawDescGZIP(), []int{9, 0}
}

func (x *ListResponse_Item) GetAction() *Action {
	if x != nil {
		return x.Action
	}
	return nil
}

func (x *ListResponse_Item) GetApprover() []*Principal {
	if x != nil {
		return x.Approver
	}
	return nil
}

func (x *ListResponse_Item) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

var File_mpa_proto protoreflect.FileDescriptor

var file_mpa_proto_rawDesc = []byte{
	0x0a, 0x09, 0x6d, 0x70, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x03, 0x4d, 0x70, 0x61,
	0x1a, 0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2f, 0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xb2, 0x01, 0x0a, 0x06,
	0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x73, 0x65, 0x72, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x75, 0x73, 0x65, 0x72, 0x12, 0x24, 0x0a, 0x0d, 0x6a, 0x75,
	0x73, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0d, 0x6a, 0x75, 0x73, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x16, 0x0a, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x12, 0x2e, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52,
	0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x26, 0x0a, 0x0f, 0x6d, 0x65, 0x74, 0x68,
	0x6f, 0x64, 0x5f, 0x77, 0x69, 0x64, 0x65, 0x5f, 0x6d, 0x70, 0x61, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x0d, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x57, 0x69, 0x64, 0x65, 0x4d, 0x70, 0x61,
	0x22, 0x33, 0x0a, 0x09, 0x50, 0x72, 0x69, 0x6e, 0x63, 0x69, 0x70, 0x61, 0x6c, 0x12, 0x0e, 0x0a,
	0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x16, 0x0a,
	0x06, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x67,
	0x72, 0x6f, 0x75, 0x70, 0x73, 0x22, 0x7e, 0x0a, 0x0c, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x12, 0x2e, 0x0a,
	0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x41, 0x6e, 0x79, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x26, 0x0a,
	0x0f, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x5f, 0x77, 0x69, 0x64, 0x65, 0x5f, 0x6d, 0x70, 0x61,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0d, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x57, 0x69,
	0x64, 0x65, 0x4d, 0x70, 0x61, 0x22, 0x70, 0x0a, 0x0d, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x23, 0x0a, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x41, 0x63, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x2a, 0x0a, 0x08, 0x61,
	0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x72, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0e, 0x2e,
	0x4d, 0x70, 0x61, 0x2e, 0x50, 0x72, 0x69, 0x6e, 0x63, 0x69, 0x70, 0x61, 0x6c, 0x52, 0x08, 0x61,
	0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x72, 0x22, 0x35, 0x0a, 0x0e, 0x41, 0x70, 0x70, 0x72, 0x6f,
	0x76, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x23, 0x0a, 0x06, 0x61, 0x63, 0x74,
	0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x4d, 0x70, 0x61, 0x2e,
	0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x11,
	0x0a, 0x0f, 0x41, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x28, 0x0a, 0x16, 0x57, 0x61, 0x69, 0x74, 0x46, 0x6f, 0x72, 0x41, 0x70, 0x70, 0x72,
	0x6f, 0x76, 0x61, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x22, 0x19, 0x0a, 0x17, 0x57,
	0x61, 0x69, 0x74, 0x46, 0x6f, 0x72, 0x41, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x61, 0x6c, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x0d, 0x0a, 0x0b, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0xa3, 0x01, 0x0a, 0x0c, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2a, 0x0a, 0x04, 0x69, 0x74, 0x65, 0x6d, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e, 0x49, 0x74, 0x65, 0x6d, 0x52, 0x04, 0x69, 0x74,
	0x65, 0x6d, 0x1a, 0x67, 0x0a, 0x04, 0x49, 0x74, 0x65, 0x6d, 0x12, 0x23, 0x0a, 0x06, 0x61, 0x63,
	0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x4d, 0x70, 0x61,
	0x2e, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12,
	0x2a, 0x0a, 0x08, 0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x72, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x0e, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x50, 0x72, 0x69, 0x6e, 0x63, 0x69, 0x70, 0x61,
	0x6c, 0x52, 0x08, 0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x72, 0x12, 0x0e, 0x0a, 0x02, 0x69,
	0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x22, 0x1c, 0x0a, 0x0a, 0x47,
	0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x22, 0x5e, 0x0a, 0x0b, 0x47, 0x65, 0x74,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x23, 0x0a, 0x06, 0x61, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x41,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x2a, 0x0a,
	0x08, 0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x72, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x0e, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x50, 0x72, 0x69, 0x6e, 0x63, 0x69, 0x70, 0x61, 0x6c, 0x52,
	0x08, 0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x72, 0x22, 0x33, 0x0a, 0x0c, 0x43, 0x6c, 0x65,
	0x61, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x23, 0x0a, 0x06, 0x61, 0x63, 0x74,
	0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x4d, 0x70, 0x61, 0x2e,
	0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x0f,
	0x0a, 0x0d, 0x43, 0x6c, 0x65, 0x61, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32,
	0xcc, 0x02, 0x0a, 0x03, 0x4d, 0x70, 0x61, 0x12, 0x30, 0x0a, 0x05, 0x53, 0x74, 0x6f, 0x72, 0x65,
	0x12, 0x11, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x12, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x36, 0x0a, 0x07, 0x41, 0x70, 0x70,
	0x72, 0x6f, 0x76, 0x65, 0x12, 0x13, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x41, 0x70, 0x70, 0x72, 0x6f,
	0x76, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x4d, 0x70, 0x61, 0x2e,
	0x41, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x4e, 0x0a, 0x0f, 0x57, 0x61, 0x69, 0x74, 0x46, 0x6f, 0x72, 0x41, 0x70, 0x70, 0x72,
	0x6f, 0x76, 0x61, 0x6c, 0x12, 0x1b, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x57, 0x61, 0x69, 0x74, 0x46,
	0x6f, 0x72, 0x41, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x61, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x1c, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x57, 0x61, 0x69, 0x74, 0x46, 0x6f, 0x72, 0x41,
	0x70, 0x70, 0x72, 0x6f, 0x76, 0x61, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x2d, 0x0a, 0x04, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x10, 0x2e, 0x4d, 0x70, 0x61, 0x2e,
	0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x11, 0x2e, 0x4d, 0x70,
	0x61, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x12, 0x2a, 0x0a, 0x03, 0x47, 0x65, 0x74, 0x12, 0x0f, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x47, 0x65,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x10, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x47,
	0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x30, 0x0a, 0x05,
	0x43, 0x6c, 0x65, 0x61, 0x72, 0x12, 0x11, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x43, 0x6c, 0x65, 0x61,
	0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x12, 0x2e, 0x4d, 0x70, 0x61, 0x2e, 0x43,
	0x6c, 0x65, 0x61, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x29,
	0x5a, 0x27, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x6e, 0x6f,
	0x77, 0x66, 0x6c, 0x61, 0x6b, 0x65, 0x2d, 0x4c, 0x61, 0x62, 0x73, 0x2f, 0x73, 0x61, 0x6e, 0x73,
	0x73, 0x68, 0x65, 0x6c, 0x6c, 0x2f, 0x6d, 0x70, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_mpa_proto_rawDescOnce sync.Once
	file_mpa_proto_rawDescData = file_mpa_proto_rawDesc
)

func file_mpa_proto_rawDescGZIP() []byte {
	file_mpa_proto_rawDescOnce.Do(func() {
		file_mpa_proto_rawDescData = protoimpl.X.CompressGZIP(file_mpa_proto_rawDescData)
	})
	return file_mpa_proto_rawDescData
}

var file_mpa_proto_msgTypes = make([]protoimpl.MessageInfo, 15)
var file_mpa_proto_goTypes = []any{
	(*Action)(nil),                  // 0: Mpa.Action
	(*Principal)(nil),               // 1: Mpa.Principal
	(*StoreRequest)(nil),            // 2: Mpa.StoreRequest
	(*StoreResponse)(nil),           // 3: Mpa.StoreResponse
	(*ApproveRequest)(nil),          // 4: Mpa.ApproveRequest
	(*ApproveResponse)(nil),         // 5: Mpa.ApproveResponse
	(*WaitForApprovalRequest)(nil),  // 6: Mpa.WaitForApprovalRequest
	(*WaitForApprovalResponse)(nil), // 7: Mpa.WaitForApprovalResponse
	(*ListRequest)(nil),             // 8: Mpa.ListRequest
	(*ListResponse)(nil),            // 9: Mpa.ListResponse
	(*GetRequest)(nil),              // 10: Mpa.GetRequest
	(*GetResponse)(nil),             // 11: Mpa.GetResponse
	(*ClearRequest)(nil),            // 12: Mpa.ClearRequest
	(*ClearResponse)(nil),           // 13: Mpa.ClearResponse
	(*ListResponse_Item)(nil),       // 14: Mpa.ListResponse.Item
	(*anypb.Any)(nil),               // 15: google.protobuf.Any
}
var file_mpa_proto_depIdxs = []int32{
	15, // 0: Mpa.Action.message:type_name -> google.protobuf.Any
	15, // 1: Mpa.StoreRequest.message:type_name -> google.protobuf.Any
	0,  // 2: Mpa.StoreResponse.action:type_name -> Mpa.Action
	1,  // 3: Mpa.StoreResponse.approver:type_name -> Mpa.Principal
	0,  // 4: Mpa.ApproveRequest.action:type_name -> Mpa.Action
	14, // 5: Mpa.ListResponse.item:type_name -> Mpa.ListResponse.Item
	0,  // 6: Mpa.GetResponse.action:type_name -> Mpa.Action
	1,  // 7: Mpa.GetResponse.approver:type_name -> Mpa.Principal
	0,  // 8: Mpa.ClearRequest.action:type_name -> Mpa.Action
	0,  // 9: Mpa.ListResponse.Item.action:type_name -> Mpa.Action
	1,  // 10: Mpa.ListResponse.Item.approver:type_name -> Mpa.Principal
	2,  // 11: Mpa.Mpa.Store:input_type -> Mpa.StoreRequest
	4,  // 12: Mpa.Mpa.Approve:input_type -> Mpa.ApproveRequest
	6,  // 13: Mpa.Mpa.WaitForApproval:input_type -> Mpa.WaitForApprovalRequest
	8,  // 14: Mpa.Mpa.List:input_type -> Mpa.ListRequest
	10, // 15: Mpa.Mpa.Get:input_type -> Mpa.GetRequest
	12, // 16: Mpa.Mpa.Clear:input_type -> Mpa.ClearRequest
	3,  // 17: Mpa.Mpa.Store:output_type -> Mpa.StoreResponse
	5,  // 18: Mpa.Mpa.Approve:output_type -> Mpa.ApproveResponse
	7,  // 19: Mpa.Mpa.WaitForApproval:output_type -> Mpa.WaitForApprovalResponse
	9,  // 20: Mpa.Mpa.List:output_type -> Mpa.ListResponse
	11, // 21: Mpa.Mpa.Get:output_type -> Mpa.GetResponse
	13, // 22: Mpa.Mpa.Clear:output_type -> Mpa.ClearResponse
	17, // [17:23] is the sub-list for method output_type
	11, // [11:17] is the sub-list for method input_type
	11, // [11:11] is the sub-list for extension type_name
	11, // [11:11] is the sub-list for extension extendee
	0,  // [0:11] is the sub-list for field type_name
}

func init() { file_mpa_proto_init() }
func file_mpa_proto_init() {
	if File_mpa_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_mpa_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Action); i {
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
		file_mpa_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*Principal); i {
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
		file_mpa_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*StoreRequest); i {
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
		file_mpa_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*StoreResponse); i {
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
		file_mpa_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*ApproveRequest); i {
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
		file_mpa_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*ApproveResponse); i {
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
		file_mpa_proto_msgTypes[6].Exporter = func(v any, i int) any {
			switch v := v.(*WaitForApprovalRequest); i {
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
		file_mpa_proto_msgTypes[7].Exporter = func(v any, i int) any {
			switch v := v.(*WaitForApprovalResponse); i {
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
		file_mpa_proto_msgTypes[8].Exporter = func(v any, i int) any {
			switch v := v.(*ListRequest); i {
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
		file_mpa_proto_msgTypes[9].Exporter = func(v any, i int) any {
			switch v := v.(*ListResponse); i {
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
		file_mpa_proto_msgTypes[10].Exporter = func(v any, i int) any {
			switch v := v.(*GetRequest); i {
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
		file_mpa_proto_msgTypes[11].Exporter = func(v any, i int) any {
			switch v := v.(*GetResponse); i {
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
		file_mpa_proto_msgTypes[12].Exporter = func(v any, i int) any {
			switch v := v.(*ClearRequest); i {
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
		file_mpa_proto_msgTypes[13].Exporter = func(v any, i int) any {
			switch v := v.(*ClearResponse); i {
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
		file_mpa_proto_msgTypes[14].Exporter = func(v any, i int) any {
			switch v := v.(*ListResponse_Item); i {
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
			RawDescriptor: file_mpa_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   15,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_mpa_proto_goTypes,
		DependencyIndexes: file_mpa_proto_depIdxs,
		MessageInfos:      file_mpa_proto_msgTypes,
	}.Build()
	File_mpa_proto = out.File
	file_mpa_proto_rawDesc = nil
	file_mpa_proto_goTypes = nil
	file_mpa_proto_depIdxs = nil
}
