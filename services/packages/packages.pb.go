// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.17.3
// source: packages.proto

package packages

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

// Allow different package systems as future proofing.
type PackageSystem int32

const (
	// The remote side will attempt to pick the appropriate one.
	PackageSystem_PACKAGE_SYSTEM_UNKNOWN PackageSystem = 0
	PackageSystem_PACKAGE_SYSTEM_YUM     PackageSystem = 1
)

// Enum value maps for PackageSystem.
var (
	PackageSystem_name = map[int32]string{
		0: "PACKAGE_SYSTEM_UNKNOWN",
		1: "PACKAGE_SYSTEM_YUM",
	}
	PackageSystem_value = map[string]int32{
		"PACKAGE_SYSTEM_UNKNOWN": 0,
		"PACKAGE_SYSTEM_YUM":     1,
	}
)

func (x PackageSystem) Enum() *PackageSystem {
	p := new(PackageSystem)
	*p = x
	return p
}

func (x PackageSystem) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PackageSystem) Descriptor() protoreflect.EnumDescriptor {
	return file_packages_proto_enumTypes[0].Descriptor()
}

func (PackageSystem) Type() protoreflect.EnumType {
	return &file_packages_proto_enumTypes[0]
}

func (x PackageSystem) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use PackageSystem.Descriptor instead.
func (PackageSystem) EnumDescriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{0}
}

type RepoStatus int32

const (
	RepoStatus_REPO_STATUS_UNKNOWN  RepoStatus = 0
	RepoStatus_REPO_STATUS_ENABLED  RepoStatus = 1
	RepoStatus_REPO_STATUS_DISABLED RepoStatus = 2
)

// Enum value maps for RepoStatus.
var (
	RepoStatus_name = map[int32]string{
		0: "REPO_STATUS_UNKNOWN",
		1: "REPO_STATUS_ENABLED",
		2: "REPO_STATUS_DISABLED",
	}
	RepoStatus_value = map[string]int32{
		"REPO_STATUS_UNKNOWN":  0,
		"REPO_STATUS_ENABLED":  1,
		"REPO_STATUS_DISABLED": 2,
	}
)

func (x RepoStatus) Enum() *RepoStatus {
	p := new(RepoStatus)
	*p = x
	return p
}

func (x RepoStatus) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (RepoStatus) Descriptor() protoreflect.EnumDescriptor {
	return file_packages_proto_enumTypes[1].Descriptor()
}

func (RepoStatus) Type() protoreflect.EnumType {
	return &file_packages_proto_enumTypes[1]
}

func (x RepoStatus) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use RepoStatus.Descriptor instead.
func (RepoStatus) EnumDescriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{1}
}

type InstallRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PackageSystem PackageSystem `protobuf:"varint,1,opt,name=package_system,json=packageSystem,proto3,enum=Packages.PackageSystem" json:"package_system,omitempty"`
	Name          string        `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Version       string        `protobuf:"bytes,3,opt,name=version,proto3" json:"version,omitempty"`
	// If set enables this repo for resolving package/version.
	Repo *string `protobuf:"bytes,4,opt,name=repo,proto3,oneof" json:"repo,omitempty"`
}

func (x *InstallRequest) Reset() {
	*x = InstallRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InstallRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InstallRequest) ProtoMessage() {}

func (x *InstallRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InstallRequest.ProtoReflect.Descriptor instead.
func (*InstallRequest) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{0}
}

func (x *InstallRequest) GetPackageSystem() PackageSystem {
	if x != nil {
		return x.PackageSystem
	}
	return PackageSystem_PACKAGE_SYSTEM_UNKNOWN
}

func (x *InstallRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *InstallRequest) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *InstallRequest) GetRepo() string {
	if x != nil && x.Repo != nil {
		return *x.Repo
	}
	return ""
}

type InstallReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DebugOutput string `protobuf:"bytes,1,opt,name=debug_output,json=debugOutput,proto3" json:"debug_output,omitempty"`
}

func (x *InstallReply) Reset() {
	*x = InstallReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InstallReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InstallReply) ProtoMessage() {}

func (x *InstallReply) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InstallReply.ProtoReflect.Descriptor instead.
func (*InstallReply) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{1}
}

func (x *InstallReply) GetDebugOutput() string {
	if x != nil {
		return x.DebugOutput
	}
	return ""
}

type UpdateRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PackageSystem PackageSystem `protobuf:"varint,1,opt,name=package_system,json=packageSystem,proto3,enum=Packages.PackageSystem" json:"package_system,omitempty"`
	Name          string        `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	// This version must be installed for update to execute.
	OldVersion string `protobuf:"bytes,3,opt,name=old_version,json=oldVersion,proto3" json:"old_version,omitempty"`
	NewVersion string `protobuf:"bytes,4,opt,name=new_version,json=newVersion,proto3" json:"new_version,omitempty"`
	// If set enables this repo as well for resolving package/version.
	Repo string `protobuf:"bytes,5,opt,name=repo,proto3" json:"repo,omitempty"`
}

func (x *UpdateRequest) Reset() {
	*x = UpdateRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateRequest) ProtoMessage() {}

func (x *UpdateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateRequest.ProtoReflect.Descriptor instead.
func (*UpdateRequest) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{2}
}

func (x *UpdateRequest) GetPackageSystem() PackageSystem {
	if x != nil {
		return x.PackageSystem
	}
	return PackageSystem_PACKAGE_SYSTEM_UNKNOWN
}

func (x *UpdateRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *UpdateRequest) GetOldVersion() string {
	if x != nil {
		return x.OldVersion
	}
	return ""
}

func (x *UpdateRequest) GetNewVersion() string {
	if x != nil {
		return x.NewVersion
	}
	return ""
}

func (x *UpdateRequest) GetRepo() string {
	if x != nil {
		return x.Repo
	}
	return ""
}

type UpdateReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DebugOutput string `protobuf:"bytes,1,opt,name=debug_output,json=debugOutput,proto3" json:"debug_output,omitempty"`
}

func (x *UpdateReply) Reset() {
	*x = UpdateReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateReply) ProtoMessage() {}

func (x *UpdateReply) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateReply.ProtoReflect.Descriptor instead.
func (*UpdateReply) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{3}
}

func (x *UpdateReply) GetDebugOutput() string {
	if x != nil {
		return x.DebugOutput
	}
	return ""
}

type ListInstalledRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PackageSystem PackageSystem `protobuf:"varint,1,opt,name=package_system,json=packageSystem,proto3,enum=Packages.PackageSystem" json:"package_system,omitempty"`
	// Takes a glob pattern in the style of the package system list command.
	GlobPattern string `protobuf:"bytes,2,opt,name=glob_pattern,json=globPattern,proto3" json:"glob_pattern,omitempty"`
}

func (x *ListInstalledRequest) Reset() {
	*x = ListInstalledRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListInstalledRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListInstalledRequest) ProtoMessage() {}

func (x *ListInstalledRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListInstalledRequest.ProtoReflect.Descriptor instead.
func (*ListInstalledRequest) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{4}
}

func (x *ListInstalledRequest) GetPackageSystem() PackageSystem {
	if x != nil {
		return x.PackageSystem
	}
	return PackageSystem_PACKAGE_SYSTEM_UNKNOWN
}

func (x *ListInstalledRequest) GetGlobPattern() string {
	if x != nil {
		return x.GlobPattern
	}
	return ""
}

type PackageInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name    string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Version string `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	Repo    string `protobuf:"bytes,3,opt,name=repo,proto3" json:"repo,omitempty"`
}

func (x *PackageInfo) Reset() {
	*x = PackageInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PackageInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PackageInfo) ProtoMessage() {}

func (x *PackageInfo) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PackageInfo.ProtoReflect.Descriptor instead.
func (*PackageInfo) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{5}
}

func (x *PackageInfo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *PackageInfo) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *PackageInfo) GetRepo() string {
	if x != nil {
		return x.Repo
	}
	return ""
}

type ListInstalledReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Packages []*PackageInfo `protobuf:"bytes,1,rep,name=packages,proto3" json:"packages,omitempty"`
}

func (x *ListInstalledReply) Reset() {
	*x = ListInstalledReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListInstalledReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListInstalledReply) ProtoMessage() {}

func (x *ListInstalledReply) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListInstalledReply.ProtoReflect.Descriptor instead.
func (*ListInstalledReply) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{6}
}

func (x *ListInstalledReply) GetPackages() []*PackageInfo {
	if x != nil {
		return x.Packages
	}
	return nil
}

type RepoListRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PackageSystem PackageSystem `protobuf:"varint,1,opt,name=package_system,json=packageSystem,proto3,enum=Packages.PackageSystem" json:"package_system,omitempty"`
}

func (x *RepoListRequest) Reset() {
	*x = RepoListRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RepoListRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RepoListRequest) ProtoMessage() {}

func (x *RepoListRequest) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RepoListRequest.ProtoReflect.Descriptor instead.
func (*RepoListRequest) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{7}
}

func (x *RepoListRequest) GetPackageSystem() PackageSystem {
	if x != nil {
		return x.PackageSystem
	}
	return PackageSystem_PACKAGE_SYSTEM_UNKNOWN
}

type Repo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id     string     `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Name   string     `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Status RepoStatus `protobuf:"varint,3,opt,name=status,proto3,enum=Packages.RepoStatus" json:"status,omitempty"`
}

func (x *Repo) Reset() {
	*x = Repo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Repo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Repo) ProtoMessage() {}

func (x *Repo) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Repo.ProtoReflect.Descriptor instead.
func (*Repo) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{8}
}

func (x *Repo) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Repo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Repo) GetStatus() RepoStatus {
	if x != nil {
		return x.Status
	}
	return RepoStatus_REPO_STATUS_UNKNOWN
}

type RepoListReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Repos []*Repo `protobuf:"bytes,1,rep,name=repos,proto3" json:"repos,omitempty"`
}

func (x *RepoListReply) Reset() {
	*x = RepoListReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_packages_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RepoListReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RepoListReply) ProtoMessage() {}

func (x *RepoListReply) ProtoReflect() protoreflect.Message {
	mi := &file_packages_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RepoListReply.ProtoReflect.Descriptor instead.
func (*RepoListReply) Descriptor() ([]byte, []int) {
	return file_packages_proto_rawDescGZIP(), []int{9}
}

func (x *RepoListReply) GetRepos() []*Repo {
	if x != nil {
		return x.Repos
	}
	return nil
}

var File_packages_proto protoreflect.FileDescriptor

var file_packages_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x08, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x22, 0xa0, 0x01, 0x0a, 0x0e, 0x49,
	0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3e, 0x0a,
	0x0e, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x73, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x17, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73,
	0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x52, 0x0d,
	0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x12, 0x12, 0x0a,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x17, 0x0a, 0x04, 0x72,
	0x65, 0x70, 0x6f, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x04, 0x72, 0x65, 0x70,
	0x6f, 0x88, 0x01, 0x01, 0x42, 0x07, 0x0a, 0x05, 0x5f, 0x72, 0x65, 0x70, 0x6f, 0x22, 0x31, 0x0a,
	0x0c, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x21, 0x0a,
	0x0c, 0x64, 0x65, 0x62, 0x75, 0x67, 0x5f, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x62, 0x75, 0x67, 0x4f, 0x75, 0x74, 0x70, 0x75, 0x74,
	0x22, 0xb9, 0x01, 0x0a, 0x0d, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x3e, 0x0a, 0x0e, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x73, 0x79,
	0x73, 0x74, 0x65, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x17, 0x2e, 0x50, 0x61, 0x63,
	0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x53, 0x79, 0x73,
	0x74, 0x65, 0x6d, 0x52, 0x0d, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x53, 0x79, 0x73, 0x74,
	0x65, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x6f, 0x6c, 0x64, 0x5f, 0x76, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x6f, 0x6c, 0x64,
	0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1f, 0x0a, 0x0b, 0x6e, 0x65, 0x77, 0x5f, 0x76,
	0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x6e, 0x65,
	0x77, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x72, 0x65, 0x70, 0x6f,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x72, 0x65, 0x70, 0x6f, 0x22, 0x30, 0x0a, 0x0b,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x21, 0x0a, 0x0c, 0x64,
	0x65, 0x62, 0x75, 0x67, 0x5f, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0b, 0x64, 0x65, 0x62, 0x75, 0x67, 0x4f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x22, 0x79,
	0x0a, 0x14, 0x4c, 0x69, 0x73, 0x74, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x65, 0x64, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3e, 0x0a, 0x0e, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67,
	0x65, 0x5f, 0x73, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x17,
	0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67,
	0x65, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x52, 0x0d, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65,
	0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x12, 0x21, 0x0a, 0x0c, 0x67, 0x6c, 0x6f, 0x62, 0x5f, 0x70,
	0x61, 0x74, 0x74, 0x65, 0x72, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x67, 0x6c,
	0x6f, 0x62, 0x50, 0x61, 0x74, 0x74, 0x65, 0x72, 0x6e, 0x22, 0x4f, 0x0a, 0x0b, 0x50, 0x61, 0x63,
	0x6b, 0x61, 0x67, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07,
	0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x76,
	0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x72, 0x65, 0x70, 0x6f, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x72, 0x65, 0x70, 0x6f, 0x22, 0x47, 0x0a, 0x12, 0x4c, 0x69,
	0x73, 0x74, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x65, 0x64, 0x52, 0x65, 0x70, 0x6c, 0x79,
	0x12, 0x31, 0x0a, 0x08, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x15, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x50, 0x61,
	0x63, 0x6b, 0x61, 0x67, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x08, 0x70, 0x61, 0x63, 0x6b, 0x61,
	0x67, 0x65, 0x73, 0x22, 0x51, 0x0a, 0x0f, 0x52, 0x65, 0x70, 0x6f, 0x4c, 0x69, 0x73, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3e, 0x0a, 0x0e, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67,
	0x65, 0x5f, 0x73, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x17,
	0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67,
	0x65, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x52, 0x0d, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65,
	0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x22, 0x58, 0x0a, 0x04, 0x52, 0x65, 0x70, 0x6f, 0x12, 0x0e,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x2c, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x14, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x52, 0x65,
	0x70, 0x6f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x22, 0x35, 0x0a, 0x0d, 0x52, 0x65, 0x70, 0x6f, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x70, 0x6c,
	0x79, 0x12, 0x24, 0x0a, 0x05, 0x72, 0x65, 0x70, 0x6f, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x0e, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x52, 0x65, 0x70, 0x6f,
	0x52, 0x05, 0x72, 0x65, 0x70, 0x6f, 0x73, 0x2a, 0x43, 0x0a, 0x0d, 0x50, 0x61, 0x63, 0x6b, 0x61,
	0x67, 0x65, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x12, 0x1a, 0x0a, 0x16, 0x50, 0x41, 0x43, 0x4b,
	0x41, 0x47, 0x45, 0x5f, 0x53, 0x59, 0x53, 0x54, 0x45, 0x4d, 0x5f, 0x55, 0x4e, 0x4b, 0x4e, 0x4f,
	0x57, 0x4e, 0x10, 0x00, 0x12, 0x16, 0x0a, 0x12, 0x50, 0x41, 0x43, 0x4b, 0x41, 0x47, 0x45, 0x5f,
	0x53, 0x59, 0x53, 0x54, 0x45, 0x4d, 0x5f, 0x59, 0x55, 0x4d, 0x10, 0x01, 0x2a, 0x58, 0x0a, 0x0a,
	0x52, 0x65, 0x70, 0x6f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x17, 0x0a, 0x13, 0x52, 0x45,
	0x50, 0x4f, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57,
	0x4e, 0x10, 0x00, 0x12, 0x17, 0x0a, 0x13, 0x52, 0x45, 0x50, 0x4f, 0x5f, 0x53, 0x54, 0x41, 0x54,
	0x55, 0x53, 0x5f, 0x45, 0x4e, 0x41, 0x42, 0x4c, 0x45, 0x44, 0x10, 0x01, 0x12, 0x18, 0x0a, 0x14,
	0x52, 0x45, 0x50, 0x4f, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x44, 0x49, 0x53, 0x41,
	0x42, 0x4c, 0x45, 0x44, 0x10, 0x02, 0x32, 0x98, 0x02, 0x0a, 0x08, 0x50, 0x61, 0x63, 0x6b, 0x61,
	0x67, 0x65, 0x73, 0x12, 0x3d, 0x0a, 0x07, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x12, 0x18,
	0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6c,
	0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61,
	0x67, 0x65, 0x73, 0x2e, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x52, 0x65, 0x70, 0x6c, 0x79,
	0x22, 0x00, 0x12, 0x3a, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x17, 0x2e, 0x50,
	0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x15, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73,
	0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x4f,
	0x0a, 0x0d, 0x4c, 0x69, 0x73, 0x74, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x65, 0x64, 0x12,
	0x1e, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x49,
	0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x65, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x1c, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x49,
	0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x65, 0x64, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12,
	0x40, 0x0a, 0x08, 0x52, 0x65, 0x70, 0x6f, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x19, 0x2e, 0x50, 0x61,
	0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x4c, 0x69, 0x73, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65,
	0x73, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22,
	0x00, 0x42, 0x34, 0x5a, 0x32, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x73, 0x6e, 0x6f, 0x77, 0x66, 0x6c, 0x61, 0x6b, 0x65, 0x64, 0x62, 0x2f, 0x75, 0x6e, 0x73, 0x68,
	0x65, 0x6c, 0x6c, 0x65, 0x64, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2f, 0x70,
	0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_packages_proto_rawDescOnce sync.Once
	file_packages_proto_rawDescData = file_packages_proto_rawDesc
)

func file_packages_proto_rawDescGZIP() []byte {
	file_packages_proto_rawDescOnce.Do(func() {
		file_packages_proto_rawDescData = protoimpl.X.CompressGZIP(file_packages_proto_rawDescData)
	})
	return file_packages_proto_rawDescData
}

var file_packages_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_packages_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_packages_proto_goTypes = []interface{}{
	(PackageSystem)(0),           // 0: Packages.PackageSystem
	(RepoStatus)(0),              // 1: Packages.RepoStatus
	(*InstallRequest)(nil),       // 2: Packages.InstallRequest
	(*InstallReply)(nil),         // 3: Packages.InstallReply
	(*UpdateRequest)(nil),        // 4: Packages.UpdateRequest
	(*UpdateReply)(nil),          // 5: Packages.UpdateReply
	(*ListInstalledRequest)(nil), // 6: Packages.ListInstalledRequest
	(*PackageInfo)(nil),          // 7: Packages.PackageInfo
	(*ListInstalledReply)(nil),   // 8: Packages.ListInstalledReply
	(*RepoListRequest)(nil),      // 9: Packages.RepoListRequest
	(*Repo)(nil),                 // 10: Packages.Repo
	(*RepoListReply)(nil),        // 11: Packages.RepoListReply
}
var file_packages_proto_depIdxs = []int32{
	0,  // 0: Packages.InstallRequest.package_system:type_name -> Packages.PackageSystem
	0,  // 1: Packages.UpdateRequest.package_system:type_name -> Packages.PackageSystem
	0,  // 2: Packages.ListInstalledRequest.package_system:type_name -> Packages.PackageSystem
	7,  // 3: Packages.ListInstalledReply.packages:type_name -> Packages.PackageInfo
	0,  // 4: Packages.RepoListRequest.package_system:type_name -> Packages.PackageSystem
	1,  // 5: Packages.Repo.status:type_name -> Packages.RepoStatus
	10, // 6: Packages.RepoListReply.repos:type_name -> Packages.Repo
	2,  // 7: Packages.Packages.Install:input_type -> Packages.InstallRequest
	4,  // 8: Packages.Packages.Update:input_type -> Packages.UpdateRequest
	6,  // 9: Packages.Packages.ListInstalled:input_type -> Packages.ListInstalledRequest
	9,  // 10: Packages.Packages.RepoList:input_type -> Packages.RepoListRequest
	3,  // 11: Packages.Packages.Install:output_type -> Packages.InstallReply
	5,  // 12: Packages.Packages.Update:output_type -> Packages.UpdateReply
	8,  // 13: Packages.Packages.ListInstalled:output_type -> Packages.ListInstalledReply
	11, // 14: Packages.Packages.RepoList:output_type -> Packages.RepoListReply
	11, // [11:15] is the sub-list for method output_type
	7,  // [7:11] is the sub-list for method input_type
	7,  // [7:7] is the sub-list for extension type_name
	7,  // [7:7] is the sub-list for extension extendee
	0,  // [0:7] is the sub-list for field type_name
}

func init() { file_packages_proto_init() }
func file_packages_proto_init() {
	if File_packages_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_packages_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InstallRequest); i {
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
		file_packages_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InstallReply); i {
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
		file_packages_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateRequest); i {
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
		file_packages_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateReply); i {
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
		file_packages_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListInstalledRequest); i {
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
		file_packages_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PackageInfo); i {
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
		file_packages_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListInstalledReply); i {
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
		file_packages_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RepoListRequest); i {
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
		file_packages_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Repo); i {
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
		file_packages_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RepoListReply); i {
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
	file_packages_proto_msgTypes[0].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_packages_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_packages_proto_goTypes,
		DependencyIndexes: file_packages_proto_depIdxs,
		EnumInfos:         file_packages_proto_enumTypes,
		MessageInfos:      file_packages_proto_msgTypes,
	}.Build()
	File_packages_proto = out.File
	file_packages_proto_rawDesc = nil
	file_packages_proto_goTypes = nil
	file_packages_proto_depIdxs = nil
}
