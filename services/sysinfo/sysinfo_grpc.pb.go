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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.23.2
// source: sysinfo.proto

package sysinfo

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
	SysInfo_Uptime_FullMethodName = "/SysInfo.SysInfo/Uptime"
)

// SysInfoClient is the client API for SysInfo service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SysInfoClient interface {
	// Uptime return
	Uptime(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*UptimeReply, error)
}

type sysInfoClient struct {
	cc grpc.ClientConnInterface
}

func NewSysInfoClient(cc grpc.ClientConnInterface) SysInfoClient {
	return &sysInfoClient{cc}
}

func (c *sysInfoClient) Uptime(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*UptimeReply, error) {
	out := new(UptimeReply)
	err := c.cc.Invoke(ctx, SysInfo_Uptime_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SysInfoServer is the server API for SysInfo service.
// All implementations should embed UnimplementedSysInfoServer
// for forward compatibility
type SysInfoServer interface {
	// Uptime return
	Uptime(context.Context, *emptypb.Empty) (*UptimeReply, error)
}

// UnimplementedSysInfoServer should be embedded to have forward compatible implementations.
type UnimplementedSysInfoServer struct {
}

func (UnimplementedSysInfoServer) Uptime(context.Context, *emptypb.Empty) (*UptimeReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Uptime not implemented")
}

// UnsafeSysInfoServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SysInfoServer will
// result in compilation errors.
type UnsafeSysInfoServer interface {
	mustEmbedUnimplementedSysInfoServer()
}

func RegisterSysInfoServer(s grpc.ServiceRegistrar, srv SysInfoServer) {
	s.RegisterService(&SysInfo_ServiceDesc, srv)
}

func _SysInfo_Uptime_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SysInfoServer).Uptime(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SysInfo_Uptime_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SysInfoServer).Uptime(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// SysInfo_ServiceDesc is the grpc.ServiceDesc for SysInfo service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SysInfo_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "SysInfo.SysInfo",
	HandlerType: (*SysInfoServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Uptime",
			Handler:    _SysInfo_Uptime_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "sysinfo.proto",
}
