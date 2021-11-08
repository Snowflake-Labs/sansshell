// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package process

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ProcessClient is the client API for Process service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ProcessClient interface {
	// List returns the output from the ps command.
	// NOTE: Since this contains the command line this can
	// contain sensitive data.
	List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListReply, error)
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
	//       the response.
	GetMemoryDump(ctx context.Context, in *GetMemoryDumpRequest, opts ...grpc.CallOption) (Process_GetMemoryDumpClient, error)
}

type processClient struct {
	cc grpc.ClientConnInterface
}

func NewProcessClient(cc grpc.ClientConnInterface) ProcessClient {
	return &processClient{cc}
}

func (c *processClient) List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListReply, error) {
	out := new(ListReply)
	err := c.cc.Invoke(ctx, "/Process.Process/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *processClient) GetStacks(ctx context.Context, in *GetStacksRequest, opts ...grpc.CallOption) (*GetStacksReply, error) {
	out := new(GetStacksReply)
	err := c.cc.Invoke(ctx, "/Process.Process/GetStacks", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *processClient) GetJavaStacks(ctx context.Context, in *GetJavaStacksRequest, opts ...grpc.CallOption) (*GetJavaStacksReply, error) {
	out := new(GetJavaStacksReply)
	err := c.cc.Invoke(ctx, "/Process.Process/GetJavaStacks", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *processClient) GetMemoryDump(ctx context.Context, in *GetMemoryDumpRequest, opts ...grpc.CallOption) (Process_GetMemoryDumpClient, error) {
	stream, err := c.cc.NewStream(ctx, &Process_ServiceDesc.Streams[0], "/Process.Process/GetMemoryDump", opts...)
	if err != nil {
		return nil, err
	}
	x := &processGetMemoryDumpClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Process_GetMemoryDumpClient interface {
	Recv() (*GetMemoryDumpReply, error)
	grpc.ClientStream
}

type processGetMemoryDumpClient struct {
	grpc.ClientStream
}

func (x *processGetMemoryDumpClient) Recv() (*GetMemoryDumpReply, error) {
	m := new(GetMemoryDumpReply)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ProcessServer is the server API for Process service.
// All implementations should embed UnimplementedProcessServer
// for forward compatibility
type ProcessServer interface {
	// List returns the output from the ps command.
	// NOTE: Since this contains the command line this can
	// contain sensitive data.
	List(context.Context, *ListRequest) (*ListReply, error)
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
	//       the response.
	GetMemoryDump(*GetMemoryDumpRequest, Process_GetMemoryDumpServer) error
}

// UnimplementedProcessServer should be embedded to have forward compatible implementations.
type UnimplementedProcessServer struct {
}

func (UnimplementedProcessServer) List(context.Context, *ListRequest) (*ListReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedProcessServer) GetStacks(context.Context, *GetStacksRequest) (*GetStacksReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStacks not implemented")
}
func (UnimplementedProcessServer) GetJavaStacks(context.Context, *GetJavaStacksRequest) (*GetJavaStacksReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetJavaStacks not implemented")
}
func (UnimplementedProcessServer) GetMemoryDump(*GetMemoryDumpRequest, Process_GetMemoryDumpServer) error {
	return status.Errorf(codes.Unimplemented, "method GetMemoryDump not implemented")
}

// UnsafeProcessServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ProcessServer will
// result in compilation errors.
type UnsafeProcessServer interface {
	mustEmbedUnimplementedProcessServer()
}

func RegisterProcessServer(s grpc.ServiceRegistrar, srv ProcessServer) {
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
		FullMethod: "/Process.Process/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProcessServer).List(ctx, req.(*ListRequest))
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
		FullMethod: "/Process.Process/GetStacks",
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
		FullMethod: "/Process.Process/GetJavaStacks",
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
	return srv.(ProcessServer).GetMemoryDump(m, &processGetMemoryDumpServer{stream})
}

type Process_GetMemoryDumpServer interface {
	Send(*GetMemoryDumpReply) error
	grpc.ServerStream
}

type processGetMemoryDumpServer struct {
	grpc.ServerStream
}

func (x *processGetMemoryDumpServer) Send(m *GetMemoryDumpReply) error {
	return x.ServerStream.SendMsg(m)
}

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
