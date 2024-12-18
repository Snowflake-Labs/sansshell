// Copyright (c) 2023 Snowflake Inc. All rights reserved.
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
// 	protoc        v5.29.1
// source: sysinfo.proto

package sysinfo

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type UptimeReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// show the uptime in protobuf.Duration
	UptimeSeconds *durationpb.Duration `protobuf:"bytes,1,opt,name=uptime_seconds,json=uptimeSeconds,proto3" json:"uptime_seconds,omitempty"`
}

func (x *UptimeReply) Reset() {
	*x = UptimeReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sysinfo_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UptimeReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UptimeReply) ProtoMessage() {}

func (x *UptimeReply) ProtoReflect() protoreflect.Message {
	mi := &file_sysinfo_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UptimeReply.ProtoReflect.Descriptor instead.
func (*UptimeReply) Descriptor() ([]byte, []int) {
	return file_sysinfo_proto_rawDescGZIP(), []int{0}
}

func (x *UptimeReply) GetUptimeSeconds() *durationpb.Duration {
	if x != nil {
		return x.UptimeSeconds
	}
	return nil
}

// DmesgRequest describes the filename to be tailed.
type DmesgRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// tail the number of line from the dmesg output, negative means display all messages
	TailLines   int32  `protobuf:"varint,1,opt,name=tail_lines,json=tailLines,proto3" json:"tail_lines,omitempty"`
	Grep        string `protobuf:"bytes,2,opt,name=grep,proto3" json:"grep,omitempty"`
	IgnoreCase  bool   `protobuf:"varint,3,opt,name=ignore_case,json=ignoreCase,proto3" json:"ignore_case,omitempty"`
	InvertMatch bool   `protobuf:"varint,4,opt,name=invert_match,json=invertMatch,proto3" json:"invert_match,omitempty"`
	// time in seconds after which dmesg output will be stopped and the result returned
	// default is 2 seconds, minimum is 2s and maximum is 30s
	Timeout int64 `protobuf:"varint,5,opt,name=timeout,proto3" json:"timeout,omitempty"`
}

func (x *DmesgRequest) Reset() {
	*x = DmesgRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sysinfo_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DmesgRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DmesgRequest) ProtoMessage() {}

func (x *DmesgRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sysinfo_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DmesgRequest.ProtoReflect.Descriptor instead.
func (*DmesgRequest) Descriptor() ([]byte, []int) {
	return file_sysinfo_proto_rawDescGZIP(), []int{1}
}

func (x *DmesgRequest) GetTailLines() int32 {
	if x != nil {
		return x.TailLines
	}
	return 0
}

func (x *DmesgRequest) GetGrep() string {
	if x != nil {
		return x.Grep
	}
	return ""
}

func (x *DmesgRequest) GetIgnoreCase() bool {
	if x != nil {
		return x.IgnoreCase
	}
	return false
}

func (x *DmesgRequest) GetInvertMatch() bool {
	if x != nil {
		return x.InvertMatch
	}
	return false
}

func (x *DmesgRequest) GetTimeout() int64 {
	if x != nil {
		return x.Timeout
	}
	return 0
}

// DmesgReply contains the messages from kernel
type DmesgReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Record *DmsgRecord `protobuf:"bytes,1,opt,name=record,proto3" json:"record,omitempty"`
}

func (x *DmesgReply) Reset() {
	*x = DmesgReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sysinfo_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DmesgReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DmesgReply) ProtoMessage() {}

func (x *DmesgReply) ProtoReflect() protoreflect.Message {
	mi := &file_sysinfo_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DmesgReply.ProtoReflect.Descriptor instead.
func (*DmesgReply) Descriptor() ([]byte, []int) {
	return file_sysinfo_proto_rawDescGZIP(), []int{2}
}

func (x *DmesgReply) GetRecord() *DmsgRecord {
	if x != nil {
		return x.Record
	}
	return nil
}

// DmesgRecord contains the specific fields about the a dmesg record
type DmsgRecord struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Time    *timestamppb.Timestamp `protobuf:"bytes,1,opt,name=time,proto3" json:"time,omitempty"`
	Message string                 `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *DmsgRecord) Reset() {
	*x = DmsgRecord{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sysinfo_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DmsgRecord) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DmsgRecord) ProtoMessage() {}

func (x *DmsgRecord) ProtoReflect() protoreflect.Message {
	mi := &file_sysinfo_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DmsgRecord.ProtoReflect.Descriptor instead.
func (*DmsgRecord) Descriptor() ([]byte, []int) {
	return file_sysinfo_proto_rawDescGZIP(), []int{3}
}

func (x *DmsgRecord) GetTime() *timestamppb.Timestamp {
	if x != nil {
		return x.Time
	}
	return nil
}

func (x *DmsgRecord) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type JournalRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The start time for query, timestamp format will be YYYY-MM-DD HH:MM:SS
	TimeSince *timestamppb.Timestamp `protobuf:"bytes,1,opt,name=time_since,json=timeSince,proto3" json:"time_since,omitempty"`
	// The end time for query
	TimeUntil *timestamppb.Timestamp `protobuf:"bytes,2,opt,name=time_until,json=timeUntil,proto3" json:"time_until,omitempty"`
	// Tail the latest number of log entries from the journal, negative means display all messages
	TailLine uint32 `protobuf:"varint,3,opt,name=tail_line,json=tailLine,proto3" json:"tail_line,omitempty"`
	// Filter messages for specified systemd units
	Unit string `protobuf:"bytes,4,opt,name=unit,proto3" json:"unit,omitempty"`
	// Controls the format of the journal entries
	EnableJson bool `protobuf:"varint,5,opt,name=enable_json,json=enableJson,proto3" json:"enable_json,omitempty"`
}

func (x *JournalRequest) Reset() {
	*x = JournalRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sysinfo_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JournalRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JournalRequest) ProtoMessage() {}

func (x *JournalRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sysinfo_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JournalRequest.ProtoReflect.Descriptor instead.
func (*JournalRequest) Descriptor() ([]byte, []int) {
	return file_sysinfo_proto_rawDescGZIP(), []int{4}
}

func (x *JournalRequest) GetTimeSince() *timestamppb.Timestamp {
	if x != nil {
		return x.TimeSince
	}
	return nil
}

func (x *JournalRequest) GetTimeUntil() *timestamppb.Timestamp {
	if x != nil {
		return x.TimeUntil
	}
	return nil
}

func (x *JournalRequest) GetTailLine() uint32 {
	if x != nil {
		return x.TailLine
	}
	return 0
}

func (x *JournalRequest) GetUnit() string {
	if x != nil {
		return x.Unit
	}
	return ""
}

func (x *JournalRequest) GetEnableJson() bool {
	if x != nil {
		return x.EnableJson
	}
	return false
}

type JournalReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Response:
	//
	//	*JournalReply_Journal
	//	*JournalReply_JournalRaw
	Response isJournalReply_Response `protobuf_oneof:"response"`
}

func (x *JournalReply) Reset() {
	*x = JournalReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sysinfo_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JournalReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JournalReply) ProtoMessage() {}

func (x *JournalReply) ProtoReflect() protoreflect.Message {
	mi := &file_sysinfo_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JournalReply.ProtoReflect.Descriptor instead.
func (*JournalReply) Descriptor() ([]byte, []int) {
	return file_sysinfo_proto_rawDescGZIP(), []int{5}
}

func (m *JournalReply) GetResponse() isJournalReply_Response {
	if m != nil {
		return m.Response
	}
	return nil
}

func (x *JournalReply) GetJournal() *JournalRecord {
	if x, ok := x.GetResponse().(*JournalReply_Journal); ok {
		return x.Journal
	}
	return nil
}

func (x *JournalReply) GetJournalRaw() *JournalRecordRaw {
	if x, ok := x.GetResponse().(*JournalReply_JournalRaw); ok {
		return x.JournalRaw
	}
	return nil
}

type isJournalReply_Response interface {
	isJournalReply_Response()
}

type JournalReply_Journal struct {
	Journal *JournalRecord `protobuf:"bytes,1,opt,name=journal,proto3,oneof"`
}

type JournalReply_JournalRaw struct {
	JournalRaw *JournalRecordRaw `protobuf:"bytes,2,opt,name=journalRaw,proto3,oneof"`
}

func (*JournalReply_Journal) isJournalReply_Response() {}

func (*JournalReply_JournalRaw) isJournalReply_Response() {}

// journal record is the default format
// and contains fields the same as journalctl output in linux
type JournalRecord struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RealtimeTimestamp *timestamppb.Timestamp `protobuf:"bytes,1,opt,name=realtime_timestamp,json=realtimeTimestamp,proto3" json:"realtime_timestamp,omitempty"`
	Hostname          string                 `protobuf:"bytes,2,opt,name=hostname,proto3" json:"hostname,omitempty"`
	SyslogIdentifier  string                 `protobuf:"bytes,3,opt,name=syslog_identifier,json=syslogIdentifier,proto3" json:"syslog_identifier,omitempty"`
	Pid               int32                  `protobuf:"varint,4,opt,name=pid,proto3" json:"pid,omitempty"`
	Message           string                 `protobuf:"bytes,5,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *JournalRecord) Reset() {
	*x = JournalRecord{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sysinfo_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JournalRecord) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JournalRecord) ProtoMessage() {}

func (x *JournalRecord) ProtoReflect() protoreflect.Message {
	mi := &file_sysinfo_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JournalRecord.ProtoReflect.Descriptor instead.
func (*JournalRecord) Descriptor() ([]byte, []int) {
	return file_sysinfo_proto_rawDescGZIP(), []int{6}
}

func (x *JournalRecord) GetRealtimeTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.RealtimeTimestamp
	}
	return nil
}

func (x *JournalRecord) GetHostname() string {
	if x != nil {
		return x.Hostname
	}
	return ""
}

func (x *JournalRecord) GetSyslogIdentifier() string {
	if x != nil {
		return x.SyslogIdentifier
	}
	return ""
}

func (x *JournalRecord) GetPid() int32 {
	if x != nil {
		return x.Pid
	}
	return 0
}

func (x *JournalRecord) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

// raw journal record will contain most fields
// and display in specified output format
type JournalRecordRaw struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// when output is set to json or json-pretty
	// a list of key-value pairs will be set here
	// the key-value pairs will be different with different messages
	Entry map[string]string `protobuf:"bytes,1,rep,name=entry,proto3" json:"entry,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *JournalRecordRaw) Reset() {
	*x = JournalRecordRaw{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sysinfo_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JournalRecordRaw) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JournalRecordRaw) ProtoMessage() {}

func (x *JournalRecordRaw) ProtoReflect() protoreflect.Message {
	mi := &file_sysinfo_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JournalRecordRaw.ProtoReflect.Descriptor instead.
func (*JournalRecordRaw) Descriptor() ([]byte, []int) {
	return file_sysinfo_proto_rawDescGZIP(), []int{7}
}

func (x *JournalRecordRaw) GetEntry() map[string]string {
	if x != nil {
		return x.Entry
	}
	return nil
}

var File_sysinfo_proto protoreflect.FileDescriptor

var file_sysinfo_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x73, 0x79, 0x73, 0x69, 0x6e, 0x66, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x53, 0x79, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x4f, 0x0a, 0x0b, 0x55, 0x70, 0x74, 0x69, 0x6d, 0x65,
	0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x40, 0x0a, 0x0e, 0x75, 0x70, 0x74, 0x69, 0x6d, 0x65, 0x5f,
	0x73, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0d, 0x75, 0x70, 0x74, 0x69, 0x6d, 0x65,
	0x53, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x22, 0x9f, 0x01, 0x0a, 0x0c, 0x44, 0x6d, 0x65, 0x73,
	0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x74, 0x61, 0x69, 0x6c,
	0x5f, 0x6c, 0x69, 0x6e, 0x65, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x74, 0x61,
	0x69, 0x6c, 0x4c, 0x69, 0x6e, 0x65, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x67, 0x72, 0x65, 0x70, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x67, 0x72, 0x65, 0x70, 0x12, 0x1f, 0x0a, 0x0b, 0x69,
	0x67, 0x6e, 0x6f, 0x72, 0x65, 0x5f, 0x63, 0x61, 0x73, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x0a, 0x69, 0x67, 0x6e, 0x6f, 0x72, 0x65, 0x43, 0x61, 0x73, 0x65, 0x12, 0x21, 0x0a, 0x0c,
	0x69, 0x6e, 0x76, 0x65, 0x72, 0x74, 0x5f, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x0b, 0x69, 0x6e, 0x76, 0x65, 0x72, 0x74, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x12,
	0x18, 0x0a, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x22, 0x39, 0x0a, 0x0a, 0x44, 0x6d, 0x65,
	0x73, 0x67, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x2b, 0x0a, 0x06, 0x72, 0x65, 0x63, 0x6f, 0x72,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x53, 0x79, 0x73, 0x49, 0x6e, 0x66,
	0x6f, 0x2e, 0x44, 0x6d, 0x73, 0x67, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x52, 0x06, 0x72, 0x65,
	0x63, 0x6f, 0x72, 0x64, 0x22, 0x56, 0x0a, 0x0a, 0x44, 0x6d, 0x73, 0x67, 0x52, 0x65, 0x63, 0x6f,
	0x72, 0x64, 0x12, 0x2e, 0x0a, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x04, 0x74, 0x69,
	0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0xd8, 0x01, 0x0a,
	0x0e, 0x4a, 0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x39, 0x0a, 0x0a, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x73, 0x69, 0x6e, 0x63, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52,
	0x09, 0x74, 0x69, 0x6d, 0x65, 0x53, 0x69, 0x6e, 0x63, 0x65, 0x12, 0x39, 0x0a, 0x0a, 0x74, 0x69,
	0x6d, 0x65, 0x5f, 0x75, 0x6e, 0x74, 0x69, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65,
	0x55, 0x6e, 0x74, 0x69, 0x6c, 0x12, 0x1b, 0x0a, 0x09, 0x74, 0x61, 0x69, 0x6c, 0x5f, 0x6c, 0x69,
	0x6e, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x08, 0x74, 0x61, 0x69, 0x6c, 0x4c, 0x69,
	0x6e, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x6e, 0x69, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x75, 0x6e, 0x69, 0x74, 0x12, 0x1f, 0x0a, 0x0b, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65,
	0x5f, 0x6a, 0x73, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a, 0x65, 0x6e, 0x61,
	0x62, 0x6c, 0x65, 0x4a, 0x73, 0x6f, 0x6e, 0x22, 0x8b, 0x01, 0x0a, 0x0c, 0x4a, 0x6f, 0x75, 0x72,
	0x6e, 0x61, 0x6c, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x32, 0x0a, 0x07, 0x6a, 0x6f, 0x75, 0x72,
	0x6e, 0x61, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x53, 0x79, 0x73, 0x49,
	0x6e, 0x66, 0x6f, 0x2e, 0x4a, 0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x52, 0x65, 0x63, 0x6f, 0x72,
	0x64, 0x48, 0x00, 0x52, 0x07, 0x6a, 0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x12, 0x3b, 0x0a, 0x0a,
	0x6a, 0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x52, 0x61, 0x77, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x19, 0x2e, 0x53, 0x79, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x2e, 0x4a, 0x6f, 0x75, 0x72, 0x6e,
	0x61, 0x6c, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x52, 0x61, 0x77, 0x48, 0x00, 0x52, 0x0a, 0x6a,
	0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x52, 0x61, 0x77, 0x42, 0x0a, 0x0a, 0x08, 0x72, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0xcf, 0x01, 0x0a, 0x0d, 0x4a, 0x6f, 0x75, 0x72, 0x6e, 0x61,
	0x6c, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x12, 0x49, 0x0a, 0x12, 0x72, 0x65, 0x61, 0x6c, 0x74,
	0x69, 0x6d, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52,
	0x11, 0x72, 0x65, 0x61, 0x6c, 0x74, 0x69, 0x6d, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x12, 0x1a, 0x0a, 0x08, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x2b,
	0x0a, 0x11, 0x73, 0x79, 0x73, 0x6c, 0x6f, 0x67, 0x5f, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66,
	0x69, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x73, 0x79, 0x73, 0x6c, 0x6f,
	0x67, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72, 0x12, 0x10, 0x0a, 0x03, 0x70,
	0x69, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03, 0x70, 0x69, 0x64, 0x12, 0x18, 0x0a,
	0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x88, 0x01, 0x0a, 0x10, 0x4a, 0x6f, 0x75, 0x72,
	0x6e, 0x61, 0x6c, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x52, 0x61, 0x77, 0x12, 0x3a, 0x0a, 0x05,
	0x65, 0x6e, 0x74, 0x72, 0x79, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x53, 0x79,
	0x73, 0x49, 0x6e, 0x66, 0x6f, 0x2e, 0x4a, 0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x52, 0x65, 0x63,
	0x6f, 0x72, 0x64, 0x52, 0x61, 0x77, 0x2e, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x52, 0x05, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x1a, 0x38, 0x0a, 0x0a, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02,
	0x38, 0x01, 0x32, 0xbb, 0x01, 0x0a, 0x07, 0x53, 0x79, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x38,
	0x0a, 0x06, 0x55, 0x70, 0x74, 0x69, 0x6d, 0x65, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79,
	0x1a, 0x14, 0x2e, 0x53, 0x79, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x2e, 0x55, 0x70, 0x74, 0x69, 0x6d,
	0x65, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x37, 0x0a, 0x05, 0x44, 0x6d, 0x65, 0x73,
	0x67, 0x12, 0x15, 0x2e, 0x53, 0x79, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x2e, 0x44, 0x6d, 0x65, 0x73,
	0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x13, 0x2e, 0x53, 0x79, 0x73, 0x49, 0x6e,
	0x66, 0x6f, 0x2e, 0x44, 0x6d, 0x65, 0x73, 0x67, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x30,
	0x01, 0x12, 0x3d, 0x0a, 0x07, 0x4a, 0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x12, 0x17, 0x2e, 0x53,
	0x79, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x2e, 0x4a, 0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x15, 0x2e, 0x53, 0x79, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x2e,
	0x4a, 0x6f, 0x75, 0x72, 0x6e, 0x61, 0x6c, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x30, 0x01,
	0x42, 0x36, 0x5a, 0x34, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53,
	0x6e, 0x6f, 0x77, 0x66, 0x6c, 0x61, 0x6b, 0x65, 0x2d, 0x4c, 0x61, 0x62, 0x73, 0x2f, 0x73, 0x61,
	0x6e, 0x73, 0x73, 0x68, 0x65, 0x6c, 0x6c, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73,
	0x2f, 0x73, 0x79, 0x73, 0x69, 0x6e, 0x66, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sysinfo_proto_rawDescOnce sync.Once
	file_sysinfo_proto_rawDescData = file_sysinfo_proto_rawDesc
)

func file_sysinfo_proto_rawDescGZIP() []byte {
	file_sysinfo_proto_rawDescOnce.Do(func() {
		file_sysinfo_proto_rawDescData = protoimpl.X.CompressGZIP(file_sysinfo_proto_rawDescData)
	})
	return file_sysinfo_proto_rawDescData
}

var file_sysinfo_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_sysinfo_proto_goTypes = []any{
	(*UptimeReply)(nil),           // 0: SysInfo.UptimeReply
	(*DmesgRequest)(nil),          // 1: SysInfo.DmesgRequest
	(*DmesgReply)(nil),            // 2: SysInfo.DmesgReply
	(*DmsgRecord)(nil),            // 3: SysInfo.DmsgRecord
	(*JournalRequest)(nil),        // 4: SysInfo.JournalRequest
	(*JournalReply)(nil),          // 5: SysInfo.JournalReply
	(*JournalRecord)(nil),         // 6: SysInfo.JournalRecord
	(*JournalRecordRaw)(nil),      // 7: SysInfo.JournalRecordRaw
	nil,                           // 8: SysInfo.JournalRecordRaw.EntryEntry
	(*durationpb.Duration)(nil),   // 9: google.protobuf.Duration
	(*timestamppb.Timestamp)(nil), // 10: google.protobuf.Timestamp
	(*emptypb.Empty)(nil),         // 11: google.protobuf.Empty
}
var file_sysinfo_proto_depIdxs = []int32{
	9,  // 0: SysInfo.UptimeReply.uptime_seconds:type_name -> google.protobuf.Duration
	3,  // 1: SysInfo.DmesgReply.record:type_name -> SysInfo.DmsgRecord
	10, // 2: SysInfo.DmsgRecord.time:type_name -> google.protobuf.Timestamp
	10, // 3: SysInfo.JournalRequest.time_since:type_name -> google.protobuf.Timestamp
	10, // 4: SysInfo.JournalRequest.time_until:type_name -> google.protobuf.Timestamp
	6,  // 5: SysInfo.JournalReply.journal:type_name -> SysInfo.JournalRecord
	7,  // 6: SysInfo.JournalReply.journalRaw:type_name -> SysInfo.JournalRecordRaw
	10, // 7: SysInfo.JournalRecord.realtime_timestamp:type_name -> google.protobuf.Timestamp
	8,  // 8: SysInfo.JournalRecordRaw.entry:type_name -> SysInfo.JournalRecordRaw.EntryEntry
	11, // 9: SysInfo.SysInfo.Uptime:input_type -> google.protobuf.Empty
	1,  // 10: SysInfo.SysInfo.Dmesg:input_type -> SysInfo.DmesgRequest
	4,  // 11: SysInfo.SysInfo.Journal:input_type -> SysInfo.JournalRequest
	0,  // 12: SysInfo.SysInfo.Uptime:output_type -> SysInfo.UptimeReply
	2,  // 13: SysInfo.SysInfo.Dmesg:output_type -> SysInfo.DmesgReply
	5,  // 14: SysInfo.SysInfo.Journal:output_type -> SysInfo.JournalReply
	12, // [12:15] is the sub-list for method output_type
	9,  // [9:12] is the sub-list for method input_type
	9,  // [9:9] is the sub-list for extension type_name
	9,  // [9:9] is the sub-list for extension extendee
	0,  // [0:9] is the sub-list for field type_name
}

func init() { file_sysinfo_proto_init() }
func file_sysinfo_proto_init() {
	if File_sysinfo_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sysinfo_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*UptimeReply); i {
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
		file_sysinfo_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*DmesgRequest); i {
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
		file_sysinfo_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*DmesgReply); i {
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
		file_sysinfo_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*DmsgRecord); i {
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
		file_sysinfo_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*JournalRequest); i {
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
		file_sysinfo_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*JournalReply); i {
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
		file_sysinfo_proto_msgTypes[6].Exporter = func(v any, i int) any {
			switch v := v.(*JournalRecord); i {
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
		file_sysinfo_proto_msgTypes[7].Exporter = func(v any, i int) any {
			switch v := v.(*JournalRecordRaw); i {
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
	file_sysinfo_proto_msgTypes[5].OneofWrappers = []any{
		(*JournalReply_Journal)(nil),
		(*JournalReply_JournalRaw)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_sysinfo_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_sysinfo_proto_goTypes,
		DependencyIndexes: file_sysinfo_proto_depIdxs,
		MessageInfos:      file_sysinfo_proto_msgTypes,
	}.Build()
	File_sysinfo_proto = out.File
	file_sysinfo_proto_rawDesc = nil
	file_sysinfo_proto_goTypes = nil
	file_sysinfo_proto_depIdxs = nil
}
