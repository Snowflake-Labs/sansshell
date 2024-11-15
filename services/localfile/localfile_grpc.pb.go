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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.28.3
// source: localfile.proto

package localfile

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	LocalFile_Read_FullMethodName              = "/LocalFile.LocalFile/Read"
	LocalFile_Stat_FullMethodName              = "/LocalFile.LocalFile/Stat"
	LocalFile_Sum_FullMethodName               = "/LocalFile.LocalFile/Sum"
	LocalFile_Write_FullMethodName             = "/LocalFile.LocalFile/Write"
	LocalFile_Copy_FullMethodName              = "/LocalFile.LocalFile/Copy"
	LocalFile_List_FullMethodName              = "/LocalFile.LocalFile/List"
	LocalFile_SetFileAttributes_FullMethodName = "/LocalFile.LocalFile/SetFileAttributes"
	LocalFile_Rm_FullMethodName                = "/LocalFile.LocalFile/Rm"
	LocalFile_Rmdir_FullMethodName             = "/LocalFile.LocalFile/Rmdir"
	LocalFile_Rename_FullMethodName            = "/LocalFile.LocalFile/Rename"
	LocalFile_Readlink_FullMethodName          = "/LocalFile.LocalFile/Readlink"
	LocalFile_Symlink_FullMethodName           = "/LocalFile.LocalFile/Symlink"
	LocalFile_Mkdir_FullMethodName             = "/LocalFile.LocalFile/Mkdir"
	LocalFile_DataGet_FullMethodName           = "/LocalFile.LocalFile/DataGet"
	LocalFile_DataSet_FullMethodName           = "/LocalFile.LocalFile/DataSet"
	LocalFile_Shred_FullMethodName             = "/LocalFile.LocalFile/Shred"
)

// LocalFileClient is the client API for LocalFile service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// The LocalFile service definition.
type LocalFileClient interface {
	// Read reads a file from the disk and returns it contents.
	Read(ctx context.Context, in *ReadActionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ReadReply], error)
	// Stat returns metadata about a filesystem path.
	Stat(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[StatRequest, StatReply], error)
	// Sum calculates a sum over the data in a single file.
	Sum(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[SumRequest, SumReply], error)
	// Write writes a file from the incoming RPC to a local file.
	Write(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[WriteRequest, emptypb.Empty], error)
	// Copy retrieves a file from the given blob URL and writes it to a local
	// file.
	Copy(ctx context.Context, in *CopyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// List returns StatReply entries for the entities contained at a given path.
	List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ListReply], error)
	// SetFileAttributes takes a given filename and sets the given attributes.
	SetFileAttributes(ctx context.Context, in *SetFileAttributesRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Rm removes the given file.
	Rm(ctx context.Context, in *RmRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Rmdir removes the given directory (must be empty).
	Rmdir(ctx context.Context, in *RmdirRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Rename renames (moves) the given file to a new name. If the destination
	// is not a directory it will be replaced if it already exists.
	// OS restrictions may apply if the old and new names are outside of the same
	// directory.
	Rename(ctx context.Context, in *RenameRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Readlink prints the value of a symbolic link
	Readlink(ctx context.Context, in *ReadlinkRequest, opts ...grpc.CallOption) (*ReadlinkReply, error)
	// Symlink creates a symbolic link
	Symlink(ctx context.Context, in *SymlinkRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Mkdir create a new directory.
	Mkdir(ctx context.Context, in *MkdirRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Get data from a file of specified type by provided key
	DataGet(ctx context.Context, in *DataGetRequest, opts ...grpc.CallOption) (*DataGetReply, error)
	// Set data value to a file of specified type by provided key
	DataSet(ctx context.Context, in *DataSetRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Perform Shred on a single file
	Shred(ctx context.Context, in *ShredRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type localFileClient struct {
	cc grpc.ClientConnInterface
}

func NewLocalFileClient(cc grpc.ClientConnInterface) LocalFileClient {
	return &localFileClient{cc}
}

func (c *localFileClient) Read(ctx context.Context, in *ReadActionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ReadReply], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[0], LocalFile_Read_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ReadActionRequest, ReadReply]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_ReadClient = grpc.ServerStreamingClient[ReadReply]

func (c *localFileClient) Stat(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[StatRequest, StatReply], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[1], LocalFile_Stat_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[StatRequest, StatReply]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_StatClient = grpc.BidiStreamingClient[StatRequest, StatReply]

func (c *localFileClient) Sum(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[SumRequest, SumReply], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[2], LocalFile_Sum_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[SumRequest, SumReply]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_SumClient = grpc.BidiStreamingClient[SumRequest, SumReply]

func (c *localFileClient) Write(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[WriteRequest, emptypb.Empty], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[3], LocalFile_Write_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[WriteRequest, emptypb.Empty]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_WriteClient = grpc.ClientStreamingClient[WriteRequest, emptypb.Empty]

func (c *localFileClient) Copy(ctx context.Context, in *CopyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_Copy_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ListReply], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[4], LocalFile_List_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ListRequest, ListReply]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_ListClient = grpc.ServerStreamingClient[ListReply]

func (c *localFileClient) SetFileAttributes(ctx context.Context, in *SetFileAttributesRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_SetFileAttributes_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) Rm(ctx context.Context, in *RmRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_Rm_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) Rmdir(ctx context.Context, in *RmdirRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_Rmdir_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) Rename(ctx context.Context, in *RenameRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_Rename_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) Readlink(ctx context.Context, in *ReadlinkRequest, opts ...grpc.CallOption) (*ReadlinkReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ReadlinkReply)
	err := c.cc.Invoke(ctx, LocalFile_Readlink_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) Symlink(ctx context.Context, in *SymlinkRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_Symlink_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) Mkdir(ctx context.Context, in *MkdirRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_Mkdir_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) DataGet(ctx context.Context, in *DataGetRequest, opts ...grpc.CallOption) (*DataGetReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DataGetReply)
	err := c.cc.Invoke(ctx, LocalFile_DataGet_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) DataSet(ctx context.Context, in *DataSetRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_DataSet_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localFileClient) Shred(ctx context.Context, in *ShredRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, LocalFile_Shred_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LocalFileServer is the server API for LocalFile service.
// All implementations should embed UnimplementedLocalFileServer
// for forward compatibility.
//
// The LocalFile service definition.
type LocalFileServer interface {
	// Read reads a file from the disk and returns it contents.
	Read(*ReadActionRequest, grpc.ServerStreamingServer[ReadReply]) error
	// Stat returns metadata about a filesystem path.
	Stat(grpc.BidiStreamingServer[StatRequest, StatReply]) error
	// Sum calculates a sum over the data in a single file.
	Sum(grpc.BidiStreamingServer[SumRequest, SumReply]) error
	// Write writes a file from the incoming RPC to a local file.
	Write(grpc.ClientStreamingServer[WriteRequest, emptypb.Empty]) error
	// Copy retrieves a file from the given blob URL and writes it to a local
	// file.
	Copy(context.Context, *CopyRequest) (*emptypb.Empty, error)
	// List returns StatReply entries for the entities contained at a given path.
	List(*ListRequest, grpc.ServerStreamingServer[ListReply]) error
	// SetFileAttributes takes a given filename and sets the given attributes.
	SetFileAttributes(context.Context, *SetFileAttributesRequest) (*emptypb.Empty, error)
	// Rm removes the given file.
	Rm(context.Context, *RmRequest) (*emptypb.Empty, error)
	// Rmdir removes the given directory (must be empty).
	Rmdir(context.Context, *RmdirRequest) (*emptypb.Empty, error)
	// Rename renames (moves) the given file to a new name. If the destination
	// is not a directory it will be replaced if it already exists.
	// OS restrictions may apply if the old and new names are outside of the same
	// directory.
	Rename(context.Context, *RenameRequest) (*emptypb.Empty, error)
	// Readlink prints the value of a symbolic link
	Readlink(context.Context, *ReadlinkRequest) (*ReadlinkReply, error)
	// Symlink creates a symbolic link
	Symlink(context.Context, *SymlinkRequest) (*emptypb.Empty, error)
	// Mkdir create a new directory.
	Mkdir(context.Context, *MkdirRequest) (*emptypb.Empty, error)
	// Get data from a file of specified type by provided key
	DataGet(context.Context, *DataGetRequest) (*DataGetReply, error)
	// Set data value to a file of specified type by provided key
	DataSet(context.Context, *DataSetRequest) (*emptypb.Empty, error)
	// Perform Shred on a single file
	Shred(context.Context, *ShredRequest) (*emptypb.Empty, error)
}

// UnimplementedLocalFileServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedLocalFileServer struct{}

func (UnimplementedLocalFileServer) Read(*ReadActionRequest, grpc.ServerStreamingServer[ReadReply]) error {
	return status.Errorf(codes.Unimplemented, "method Read not implemented")
}
func (UnimplementedLocalFileServer) Stat(grpc.BidiStreamingServer[StatRequest, StatReply]) error {
	return status.Errorf(codes.Unimplemented, "method Stat not implemented")
}
func (UnimplementedLocalFileServer) Sum(grpc.BidiStreamingServer[SumRequest, SumReply]) error {
	return status.Errorf(codes.Unimplemented, "method Sum not implemented")
}
func (UnimplementedLocalFileServer) Write(grpc.ClientStreamingServer[WriteRequest, emptypb.Empty]) error {
	return status.Errorf(codes.Unimplemented, "method Write not implemented")
}
func (UnimplementedLocalFileServer) Copy(context.Context, *CopyRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Copy not implemented")
}
func (UnimplementedLocalFileServer) List(*ListRequest, grpc.ServerStreamingServer[ListReply]) error {
	return status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedLocalFileServer) SetFileAttributes(context.Context, *SetFileAttributesRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetFileAttributes not implemented")
}
func (UnimplementedLocalFileServer) Rm(context.Context, *RmRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Rm not implemented")
}
func (UnimplementedLocalFileServer) Rmdir(context.Context, *RmdirRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Rmdir not implemented")
}
func (UnimplementedLocalFileServer) Rename(context.Context, *RenameRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Rename not implemented")
}
func (UnimplementedLocalFileServer) Readlink(context.Context, *ReadlinkRequest) (*ReadlinkReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Readlink not implemented")
}
func (UnimplementedLocalFileServer) Symlink(context.Context, *SymlinkRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Symlink not implemented")
}
func (UnimplementedLocalFileServer) Mkdir(context.Context, *MkdirRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Mkdir not implemented")
}
func (UnimplementedLocalFileServer) DataGet(context.Context, *DataGetRequest) (*DataGetReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DataGet not implemented")
}
func (UnimplementedLocalFileServer) DataSet(context.Context, *DataSetRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DataSet not implemented")
}
func (UnimplementedLocalFileServer) Shred(context.Context, *ShredRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Shred not implemented")
}
func (UnimplementedLocalFileServer) testEmbeddedByValue() {}

// UnsafeLocalFileServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to LocalFileServer will
// result in compilation errors.
type UnsafeLocalFileServer interface {
	mustEmbedUnimplementedLocalFileServer()
}

func RegisterLocalFileServer(s grpc.ServiceRegistrar, srv LocalFileServer) {
	// If the following call pancis, it indicates UnimplementedLocalFileServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&LocalFile_ServiceDesc, srv)
}

func _LocalFile_Read_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ReadActionRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(LocalFileServer).Read(m, &grpc.GenericServerStream[ReadActionRequest, ReadReply]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_ReadServer = grpc.ServerStreamingServer[ReadReply]

func _LocalFile_Stat_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(LocalFileServer).Stat(&grpc.GenericServerStream[StatRequest, StatReply]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_StatServer = grpc.BidiStreamingServer[StatRequest, StatReply]

func _LocalFile_Sum_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(LocalFileServer).Sum(&grpc.GenericServerStream[SumRequest, SumReply]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_SumServer = grpc.BidiStreamingServer[SumRequest, SumReply]

func _LocalFile_Write_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(LocalFileServer).Write(&grpc.GenericServerStream[WriteRequest, emptypb.Empty]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_WriteServer = grpc.ClientStreamingServer[WriteRequest, emptypb.Empty]

func _LocalFile_Copy_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CopyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).Copy(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_Copy_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).Copy(ctx, req.(*CopyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_List_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ListRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(LocalFileServer).List(m, &grpc.GenericServerStream[ListRequest, ListReply]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LocalFile_ListServer = grpc.ServerStreamingServer[ListReply]

func _LocalFile_SetFileAttributes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetFileAttributesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).SetFileAttributes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_SetFileAttributes_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).SetFileAttributes(ctx, req.(*SetFileAttributesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_Rm_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RmRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).Rm(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_Rm_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).Rm(ctx, req.(*RmRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_Rmdir_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RmdirRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).Rmdir(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_Rmdir_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).Rmdir(ctx, req.(*RmdirRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_Rename_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RenameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).Rename(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_Rename_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).Rename(ctx, req.(*RenameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_Readlink_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadlinkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).Readlink(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_Readlink_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).Readlink(ctx, req.(*ReadlinkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_Symlink_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SymlinkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).Symlink(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_Symlink_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).Symlink(ctx, req.(*SymlinkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_Mkdir_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MkdirRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).Mkdir(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_Mkdir_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).Mkdir(ctx, req.(*MkdirRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_DataGet_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DataGetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).DataGet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_DataGet_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).DataGet(ctx, req.(*DataGetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_DataSet_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DataSetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).DataSet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_DataSet_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).DataSet(ctx, req.(*DataSetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LocalFile_Shred_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ShredRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LocalFileServer).Shred(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LocalFile_Shred_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LocalFileServer).Shred(ctx, req.(*ShredRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// LocalFile_ServiceDesc is the grpc.ServiceDesc for LocalFile service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var LocalFile_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "LocalFile.LocalFile",
	HandlerType: (*LocalFileServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Copy",
			Handler:    _LocalFile_Copy_Handler,
		},
		{
			MethodName: "SetFileAttributes",
			Handler:    _LocalFile_SetFileAttributes_Handler,
		},
		{
			MethodName: "Rm",
			Handler:    _LocalFile_Rm_Handler,
		},
		{
			MethodName: "Rmdir",
			Handler:    _LocalFile_Rmdir_Handler,
		},
		{
			MethodName: "Rename",
			Handler:    _LocalFile_Rename_Handler,
		},
		{
			MethodName: "Readlink",
			Handler:    _LocalFile_Readlink_Handler,
		},
		{
			MethodName: "Symlink",
			Handler:    _LocalFile_Symlink_Handler,
		},
		{
			MethodName: "Mkdir",
			Handler:    _LocalFile_Mkdir_Handler,
		},
		{
			MethodName: "DataGet",
			Handler:    _LocalFile_DataGet_Handler,
		},
		{
			MethodName: "DataSet",
			Handler:    _LocalFile_DataSet_Handler,
		},
		{
			MethodName: "Shred",
			Handler:    _LocalFile_Shred_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Read",
			Handler:       _LocalFile_Read_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Stat",
			Handler:       _LocalFile_Stat_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Sum",
			Handler:       _LocalFile_Sum_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Write",
			Handler:       _LocalFile_Write_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "List",
			Handler:       _LocalFile_List_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "localfile.proto",
}
