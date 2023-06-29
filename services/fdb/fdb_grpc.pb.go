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
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.23.3
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
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Conf_Read_FullMethodName   = "/Fdb.Conf/Read"
	Conf_Write_FullMethodName  = "/Fdb.Conf/Write"
	Conf_Delete_FullMethodName = "/Fdb.Conf/Delete"
)

// ConfClient is the client API for Conf service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
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
	out := new(FdbConfResponse)
	err := c.cc.Invoke(ctx, Conf_Read_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *confClient) Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Conf_Write_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *confClient) Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Conf_Delete_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConfServer is the server API for Conf service.
// All implementations should embed UnimplementedConfServer
// for forward compatibility
type ConfServer interface {
	// Read a configuration value from a section in FDB config file.
	Read(context.Context, *ReadRequest) (*FdbConfResponse, error)
	// Write updates a configuration value in a section of FDB config file.
	Write(context.Context, *WriteRequest) (*emptypb.Empty, error)
	// Delete a section value based on a key
	Delete(context.Context, *DeleteRequest) (*emptypb.Empty, error)
}

// UnimplementedConfServer should be embedded to have forward compatible implementations.
type UnimplementedConfServer struct {
}

func (UnimplementedConfServer) Read(context.Context, *ReadRequest) (*FdbConfResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Read not implemented")
}
func (UnimplementedConfServer) Write(context.Context, *WriteRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Write not implemented")
}
func (UnimplementedConfServer) Delete(context.Context, *DeleteRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}

// UnsafeConfServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ConfServer will
// result in compilation errors.
type UnsafeConfServer interface {
	mustEmbedUnimplementedConfServer()
}

func RegisterConfServer(s grpc.ServiceRegistrar, srv ConfServer) {
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
	CLI_FDBCLI_FullMethodName = "/Fdb.CLI/FDBCLI"
)

// CLIClient is the client API for CLI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type CLIClient interface {
	FDBCLI(ctx context.Context, in *FDBCLIRequest, opts ...grpc.CallOption) (CLI_FDBCLIClient, error)
}

type cLIClient struct {
	cc grpc.ClientConnInterface
}

func NewCLIClient(cc grpc.ClientConnInterface) CLIClient {
	return &cLIClient{cc}
}

func (c *cLIClient) FDBCLI(ctx context.Context, in *FDBCLIRequest, opts ...grpc.CallOption) (CLI_FDBCLIClient, error) {
	stream, err := c.cc.NewStream(ctx, &CLI_ServiceDesc.Streams[0], CLI_FDBCLI_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &cLIFDBCLIClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type CLI_FDBCLIClient interface {
	Recv() (*FDBCLIResponse, error)
	grpc.ClientStream
}

type cLIFDBCLIClient struct {
	grpc.ClientStream
}

func (x *cLIFDBCLIClient) Recv() (*FDBCLIResponse, error) {
	m := new(FDBCLIResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// CLIServer is the server API for CLI service.
// All implementations should embed UnimplementedCLIServer
// for forward compatibility
type CLIServer interface {
	FDBCLI(*FDBCLIRequest, CLI_FDBCLIServer) error
}

// UnimplementedCLIServer should be embedded to have forward compatible implementations.
type UnimplementedCLIServer struct {
}

func (UnimplementedCLIServer) FDBCLI(*FDBCLIRequest, CLI_FDBCLIServer) error {
	return status.Errorf(codes.Unimplemented, "method FDBCLI not implemented")
}

// UnsafeCLIServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to CLIServer will
// result in compilation errors.
type UnsafeCLIServer interface {
	mustEmbedUnimplementedCLIServer()
}

func RegisterCLIServer(s grpc.ServiceRegistrar, srv CLIServer) {
	s.RegisterService(&CLI_ServiceDesc, srv)
}

func _CLI_FDBCLI_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(FDBCLIRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(CLIServer).FDBCLI(m, &cLIFDBCLIServer{stream})
}

type CLI_FDBCLIServer interface {
	Send(*FDBCLIResponse) error
	grpc.ServerStream
}

type cLIFDBCLIServer struct {
	grpc.ServerStream
}

func (x *cLIFDBCLIServer) Send(m *FDBCLIResponse) error {
	return x.ServerStream.SendMsg(m)
}

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
	out := new(FDBServerResponse)
	err := c.cc.Invoke(ctx, Server_FDBServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ServerServer is the server API for Server service.
// All implementations should embed UnimplementedServerServer
// for forward compatibility
type ServerServer interface {
	FDBServer(context.Context, *FDBServerRequest) (*FDBServerResponse, error)
}

// UnimplementedServerServer should be embedded to have forward compatible implementations.
type UnimplementedServerServer struct {
}

func (UnimplementedServerServer) FDBServer(context.Context, *FDBServerRequest) (*FDBServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FDBServer not implemented")
}

// UnsafeServerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ServerServer will
// result in compilation errors.
type UnsafeServerServer interface {
	mustEmbedUnimplementedServerServer()
}

func RegisterServerServer(s grpc.ServiceRegistrar, srv ServerServer) {
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
