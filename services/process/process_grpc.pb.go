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
// source: process.proto

package process

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
	Process_List_FullMethodName          = "/Process.Process/List"
	Process_Kill_FullMethodName          = "/Process.Process/Kill"
	Process_GetStacks_FullMethodName     = "/Process.Process/GetStacks"
	Process_GetJavaStacks_FullMethodName = "/Process.Process/GetJavaStacks"
	Process_GetMemoryDump_FullMethodName = "/Process.Process/GetMemoryDump"
)

// ProcessClient is the client API for Process service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// The Process service definition.
type ProcessClient interface {
	// List returns the output from the ps command.
	// NOTE: Since this contains the command line this can
	// contain sensitive data.
	List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListReply, error)
	// Kill will send a signal to the given process id and return its status via
	// error handling.
	Kill(ctx context.Context, in *KillRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// GetStacks will return the output from pstack which generally has nothing
	// sensitive in it but depending on function names could have internal details
	// so be careful.
	GetStacks(ctx context.Context, in *GetStacksRequest, opts ...grpc.CallOption) (*GetStacksReply, error)
	// GetJavaStacks will return the output from jstack which generally has
	// nothing sensitive in it but depending on function names could have internal
	// details so be careful.
	GetJavaStacks(ctx context.Context, in *GetJavaStacksRequest, opts ...grpc.CallOption) (*GetJavaStacksReply, error)
	// GetMemoryDump will return the output from gcore or jmap which 100% has
	// sensitive data contained within it. Be very careful where this is
	// stored/transferred/etc.
	// NOTE: Enough disk space is required to hold the dump file before streaming
	//
	//	the response.
	GetMemoryDump(ctx context.Context, in *GetMemoryDumpRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetMemoryDumpReply], error)
}

type processClient struct {
	cc grpc.ClientConnInterface
}

func NewProcessClient(cc grpc.ClientConnInterface) ProcessClient {
	return &processClient{cc}
}

func (c *processClient) List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListReply)
	err := c.cc.Invoke(ctx, Process_List_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *processClient) Kill(ctx context.Context, in *KillRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Process_Kill_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *processClient) GetStacks(ctx context.Context, in *GetStacksRequest, opts ...grpc.CallOption) (*GetStacksReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetStacksReply)
	err := c.cc.Invoke(ctx, Process_GetStacks_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *processClient) GetJavaStacks(ctx context.Context, in *GetJavaStacksRequest, opts ...grpc.CallOption) (*GetJavaStacksReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetJavaStacksReply)
	err := c.cc.Invoke(ctx, Process_GetJavaStacks_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *processClient) GetMemoryDump(ctx context.Context, in *GetMemoryDumpRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetMemoryDumpReply], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Process_ServiceDesc.Streams[0], Process_GetMemoryDump_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GetMemoryDumpRequest, GetMemoryDumpReply]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Process_GetMemoryDumpClient = grpc.ServerStreamingClient[GetMemoryDumpReply]

// ProcessServer is the server API for Process service.
// All implementations should embed UnimplementedProcessServer
// for forward compatibility.
//
// The Process service definition.
type ProcessServer interface {
	// List returns the output from the ps command.
	// NOTE: Since this contains the command line this can
	// contain sensitive data.
	List(context.Context, *ListRequest) (*ListReply, error)
	// Kill will send a signal to the given process id and return its status via
	// error handling.
	Kill(context.Context, *KillRequest) (*emptypb.Empty, error)
	// GetStacks will return the output from pstack which generally has nothing
	// sensitive in it but depending on function names could have internal details
	// so be careful.
	GetStacks(context.Context, *GetStacksRequest) (*GetStacksReply, error)
	// GetJavaStacks will return the output from jstack which generally has
	// nothing sensitive in it but depending on function names could have internal
	// details so be careful.
	GetJavaStacks(context.Context, *GetJavaStacksRequest) (*GetJavaStacksReply, error)
	// GetMemoryDump will return the output from gcore or jmap which 100% has
	// sensitive data contained within it. Be very careful where this is
	// stored/transferred/etc.
	// NOTE: Enough disk space is required to hold the dump file before streaming
	//
	//	the response.
	GetMemoryDump(*GetMemoryDumpRequest, grpc.ServerStreamingServer[GetMemoryDumpReply]) error
}

// UnimplementedProcessServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedProcessServer struct{}

func (UnimplementedProcessServer) List(context.Context, *ListRequest) (*ListReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedProcessServer) Kill(context.Context, *KillRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Kill not implemented")
}
func (UnimplementedProcessServer) GetStacks(context.Context, *GetStacksRequest) (*GetStacksReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStacks not implemented")
}
func (UnimplementedProcessServer) GetJavaStacks(context.Context, *GetJavaStacksRequest) (*GetJavaStacksReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetJavaStacks not implemented")
}
func (UnimplementedProcessServer) GetMemoryDump(*GetMemoryDumpRequest, grpc.ServerStreamingServer[GetMemoryDumpReply]) error {
	return status.Errorf(codes.Unimplemented, "method GetMemoryDump not implemented")
}
func (UnimplementedProcessServer) testEmbeddedByValue() {}

// UnsafeProcessServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ProcessServer will
// result in compilation errors.
type UnsafeProcessServer interface {
	mustEmbedUnimplementedProcessServer()
}

func RegisterProcessServer(s grpc.ServiceRegistrar, srv ProcessServer) {
	// If the following call pancis, it indicates UnimplementedProcessServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Process_ServiceDesc, srv)
}

func _Process_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProcessServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Process_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProcessServer).List(ctx, req.(*ListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Process_Kill_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KillRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProcessServer).Kill(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Process_Kill_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProcessServer).Kill(ctx, req.(*KillRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Process_GetStacks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetStacksRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProcessServer).GetStacks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Process_GetStacks_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProcessServer).GetStacks(ctx, req.(*GetStacksRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Process_GetJavaStacks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetJavaStacksRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProcessServer).GetJavaStacks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Process_GetJavaStacks_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProcessServer).GetJavaStacks(ctx, req.(*GetJavaStacksRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Process_GetMemoryDump_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetMemoryDumpRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ProcessServer).GetMemoryDump(m, &grpc.GenericServerStream[GetMemoryDumpRequest, GetMemoryDumpReply]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Process_GetMemoryDumpServer = grpc.ServerStreamingServer[GetMemoryDumpReply]

// Process_ServiceDesc is the grpc.ServiceDesc for Process service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Process_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Process.Process",
	HandlerType: (*ProcessServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "List",
			Handler:    _Process_List_Handler,
		},
		{
			MethodName: "Kill",
			Handler:    _Process_Kill_Handler,
		},
		{
			MethodName: "GetStacks",
			Handler:    _Process_GetStacks_Handler,
		},
		{
			MethodName: "GetJavaStacks",
			Handler:    _Process_GetJavaStacks_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetMemoryDump",
			Handler:       _Process_GetMemoryDump_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "process.proto",
}
