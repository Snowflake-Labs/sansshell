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
// - protoc             v5.29.3
// source: fdb.proto

package fdb

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
	Conf_Read_FullMethodName   = "/Fdb.Conf/Read"
	Conf_Write_FullMethodName  = "/Fdb.Conf/Write"
	Conf_Delete_FullMethodName = "/Fdb.Conf/Delete"
)

// ConfClient is the client API for Conf service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// The fdb configuration service definition.
type ConfClient interface {
	// Read a configuration value from a section in FDB config file.
	Read(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (*FdbConfResponse, error)
	// Write updates a configuration value in a section of FDB config file.
	Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Delete a section value based on a key
	Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type confClient struct {
	cc grpc.ClientConnInterface
}

func NewConfClient(cc grpc.ClientConnInterface) ConfClient {
	return &confClient{cc}
}

func (c *confClient) Read(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (*FdbConfResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(FdbConfResponse)
	err := c.cc.Invoke(ctx, Conf_Read_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *confClient) Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Conf_Write_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *confClient) Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Conf_Delete_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConfServer is the server API for Conf service.
// All implementations should embed UnimplementedConfServer
// for forward compatibility.
//
// The fdb configuration service definition.
type ConfServer interface {
	// Read a configuration value from a section in FDB config file.
	Read(context.Context, *ReadRequest) (*FdbConfResponse, error)
	// Write updates a configuration value in a section of FDB config file.
	Write(context.Context, *WriteRequest) (*emptypb.Empty, error)
	// Delete a section value based on a key
	Delete(context.Context, *DeleteRequest) (*emptypb.Empty, error)
}

// UnimplementedConfServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedConfServer struct{}

func (UnimplementedConfServer) Read(context.Context, *ReadRequest) (*FdbConfResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Read not implemented")
}
func (UnimplementedConfServer) Write(context.Context, *WriteRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Write not implemented")
}
func (UnimplementedConfServer) Delete(context.Context, *DeleteRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedConfServer) testEmbeddedByValue() {}

// UnsafeConfServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ConfServer will
// result in compilation errors.
type UnsafeConfServer interface {
	mustEmbedUnimplementedConfServer()
}

func RegisterConfServer(s grpc.ServiceRegistrar, srv ConfServer) {
	// If the following call pancis, it indicates UnimplementedConfServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Conf_ServiceDesc, srv)
}

func _Conf_Read_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfServer).Read(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Conf_Read_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfServer).Read(ctx, req.(*ReadRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Conf_Write_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(WriteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfServer).Write(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Conf_Write_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfServer).Write(ctx, req.(*WriteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Conf_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Conf_Delete_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfServer).Delete(ctx, req.(*DeleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Conf_ServiceDesc is the grpc.ServiceDesc for Conf service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Conf_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Fdb.Conf",
	HandlerType: (*ConfServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Read",
			Handler:    _Conf_Read_Handler,
		},
		{
			MethodName: "Write",
			Handler:    _Conf_Write_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _Conf_Delete_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "fdb.proto",
}

const (
	FDBMove_FDBMoveDataCopy_FullMethodName = "/Fdb.FDBMove/FDBMoveDataCopy"
	FDBMove_FDBMoveDataWait_FullMethodName = "/Fdb.FDBMove/FDBMoveDataWait"
)

// FDBMoveClient is the client API for FDBMove service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type FDBMoveClient interface {
	FDBMoveDataCopy(ctx context.Context, in *FDBMoveDataCopyRequest, opts ...grpc.CallOption) (*FDBMoveDataCopyResponse, error)
	FDBMoveDataWait(ctx context.Context, in *FDBMoveDataWaitRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[FDBMoveDataWaitResponse], error)
}

type fDBMoveClient struct {
	cc grpc.ClientConnInterface
}

func NewFDBMoveClient(cc grpc.ClientConnInterface) FDBMoveClient {
	return &fDBMoveClient{cc}
}

func (c *fDBMoveClient) FDBMoveDataCopy(ctx context.Context, in *FDBMoveDataCopyRequest, opts ...grpc.CallOption) (*FDBMoveDataCopyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(FDBMoveDataCopyResponse)
	err := c.cc.Invoke(ctx, FDBMove_FDBMoveDataCopy_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *fDBMoveClient) FDBMoveDataWait(ctx context.Context, in *FDBMoveDataWaitRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[FDBMoveDataWaitResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &FDBMove_ServiceDesc.Streams[0], FDBMove_FDBMoveDataWait_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[FDBMoveDataWaitRequest, FDBMoveDataWaitResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type FDBMove_FDBMoveDataWaitClient = grpc.ServerStreamingClient[FDBMoveDataWaitResponse]

// FDBMoveServer is the server API for FDBMove service.
// All implementations should embed UnimplementedFDBMoveServer
// for forward compatibility.
type FDBMoveServer interface {
	FDBMoveDataCopy(context.Context, *FDBMoveDataCopyRequest) (*FDBMoveDataCopyResponse, error)
	FDBMoveDataWait(*FDBMoveDataWaitRequest, grpc.ServerStreamingServer[FDBMoveDataWaitResponse]) error
}

// UnimplementedFDBMoveServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedFDBMoveServer struct{}

func (UnimplementedFDBMoveServer) FDBMoveDataCopy(context.Context, *FDBMoveDataCopyRequest) (*FDBMoveDataCopyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FDBMoveDataCopy not implemented")
}
func (UnimplementedFDBMoveServer) FDBMoveDataWait(*FDBMoveDataWaitRequest, grpc.ServerStreamingServer[FDBMoveDataWaitResponse]) error {
	return status.Errorf(codes.Unimplemented, "method FDBMoveDataWait not implemented")
}
func (UnimplementedFDBMoveServer) testEmbeddedByValue() {}

// UnsafeFDBMoveServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to FDBMoveServer will
// result in compilation errors.
type UnsafeFDBMoveServer interface {
	mustEmbedUnimplementedFDBMoveServer()
}

func RegisterFDBMoveServer(s grpc.ServiceRegistrar, srv FDBMoveServer) {
	// If the following call pancis, it indicates UnimplementedFDBMoveServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&FDBMove_ServiceDesc, srv)
}

func _FDBMove_FDBMoveDataCopy_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FDBMoveDataCopyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FDBMoveServer).FDBMoveDataCopy(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: FDBMove_FDBMoveDataCopy_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FDBMoveServer).FDBMoveDataCopy(ctx, req.(*FDBMoveDataCopyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _FDBMove_FDBMoveDataWait_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(FDBMoveDataWaitRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(FDBMoveServer).FDBMoveDataWait(m, &grpc.GenericServerStream[FDBMoveDataWaitRequest, FDBMoveDataWaitResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type FDBMove_FDBMoveDataWaitServer = grpc.ServerStreamingServer[FDBMoveDataWaitResponse]

// FDBMove_ServiceDesc is the grpc.ServiceDesc for FDBMove service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var FDBMove_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Fdb.FDBMove",
	HandlerType: (*FDBMoveServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "FDBMoveDataCopy",
			Handler:    _FDBMove_FDBMoveDataCopy_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "FDBMoveDataWait",
			Handler:       _FDBMove_FDBMoveDataWait_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "fdb.proto",
}

const (
	CLI_FDBCLI_FullMethodName = "/Fdb.CLI/FDBCLI"
)

// CLIClient is the client API for CLI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type CLIClient interface {
	FDBCLI(ctx context.Context, in *FDBCLIRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[FDBCLIResponse], error)
}

type cLIClient struct {
	cc grpc.ClientConnInterface
}

func NewCLIClient(cc grpc.ClientConnInterface) CLIClient {
	return &cLIClient{cc}
}

func (c *cLIClient) FDBCLI(ctx context.Context, in *FDBCLIRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[FDBCLIResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &CLI_ServiceDesc.Streams[0], CLI_FDBCLI_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[FDBCLIRequest, FDBCLIResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type CLI_FDBCLIClient = grpc.ServerStreamingClient[FDBCLIResponse]

// CLIServer is the server API for CLI service.
// All implementations should embed UnimplementedCLIServer
// for forward compatibility.
type CLIServer interface {
	FDBCLI(*FDBCLIRequest, grpc.ServerStreamingServer[FDBCLIResponse]) error
}

// UnimplementedCLIServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedCLIServer struct{}

func (UnimplementedCLIServer) FDBCLI(*FDBCLIRequest, grpc.ServerStreamingServer[FDBCLIResponse]) error {
	return status.Errorf(codes.Unimplemented, "method FDBCLI not implemented")
}
func (UnimplementedCLIServer) testEmbeddedByValue() {}

// UnsafeCLIServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to CLIServer will
// result in compilation errors.
type UnsafeCLIServer interface {
	mustEmbedUnimplementedCLIServer()
}

func RegisterCLIServer(s grpc.ServiceRegistrar, srv CLIServer) {
	// If the following call pancis, it indicates UnimplementedCLIServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&CLI_ServiceDesc, srv)
}

func _CLI_FDBCLI_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(FDBCLIRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(CLIServer).FDBCLI(m, &grpc.GenericServerStream[FDBCLIRequest, FDBCLIResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type CLI_FDBCLIServer = grpc.ServerStreamingServer[FDBCLIResponse]

// CLI_ServiceDesc is the grpc.ServiceDesc for CLI service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var CLI_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Fdb.CLI",
	HandlerType: (*CLIServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "FDBCLI",
			Handler:       _CLI_FDBCLI_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "fdb.proto",
}

const (
	Server_FDBServer_FullMethodName = "/Fdb.Server/FDBServer"
)

// ServerClient is the client API for Server service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ServerClient interface {
	FDBServer(ctx context.Context, in *FDBServerRequest, opts ...grpc.CallOption) (*FDBServerResponse, error)
}

type serverClient struct {
	cc grpc.ClientConnInterface
}

func NewServerClient(cc grpc.ClientConnInterface) ServerClient {
	return &serverClient{cc}
}

func (c *serverClient) FDBServer(ctx context.Context, in *FDBServerRequest, opts ...grpc.CallOption) (*FDBServerResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(FDBServerResponse)
	err := c.cc.Invoke(ctx, Server_FDBServer_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ServerServer is the server API for Server service.
// All implementations should embed UnimplementedServerServer
// for forward compatibility.
type ServerServer interface {
	FDBServer(context.Context, *FDBServerRequest) (*FDBServerResponse, error)
}

// UnimplementedServerServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedServerServer struct{}

func (UnimplementedServerServer) FDBServer(context.Context, *FDBServerRequest) (*FDBServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FDBServer not implemented")
}
func (UnimplementedServerServer) testEmbeddedByValue() {}

// UnsafeServerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ServerServer will
// result in compilation errors.
type UnsafeServerServer interface {
	mustEmbedUnimplementedServerServer()
}

func RegisterServerServer(s grpc.ServiceRegistrar, srv ServerServer) {
	// If the following call pancis, it indicates UnimplementedServerServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Server_ServiceDesc, srv)
}

func _Server_FDBServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FDBServerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerServer).FDBServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Server_FDBServer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerServer).FDBServer(ctx, req.(*FDBServerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Server_ServiceDesc is the grpc.ServiceDesc for Server service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Server_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Fdb.Server",
	HandlerType: (*ServerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "FDBServer",
			Handler:    _Server_FDBServer_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "fdb.proto",
}
