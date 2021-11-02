// Package proxy provides the client side API for working with a proxy server.
package proxy

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

type ProxyConn struct {
	// The targets we're proxying for currently.
	targets []string
	// The RPC connection to the proxy.
	cc *grpc.ClientConn
}

// Invoke implements grpc.ClientConnInterface
func (p *ProxyConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// NewStream implements grpc.ClientConnInterface
func (p *ProxyConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// Additional methods needed for 1:N and N:N below.
type ProxyRet struct {
	Target string
	Ret    anypb.Any
	Error  error
}

func (p *ProxyConn) InvokeOneMany(ctx context.Context, method string, args interface{}, opts ...grpc.CallOption) (chan *ProxyRet, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

type ProxyMany struct {
	Target string
	Method string
	Args   interface{}
}

func (p *ProxyConn) InvokeManyMany(ctx context.Context, calls []ProxyMany, opts ...grpc.CallOption) (chan *ProxyRet, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (p *ProxyConn) Close() error {
	return p.cc.Close()
}

// Dial will connect to the given proxy and setup to send RPCs to the listed targets.
// If proxy is blank and there is only one target this will return a normal grpc connection object (*grpc.ClientConn).
// Otherwise this will return a *ProxyConn setup to act with the proxy.
func Dial(proxy string, targets []string, opts ...grpc.DialOption) (grpc.ClientConnInterface, error) {
	if proxy == "" {
		if len(targets) != 1 {
			return nil, status.Error(codes.InvalidArgument, "no proxy specified but more than one target set")
		}
		return grpc.Dial(targets[0], opts...)
	}
	conn, err := grpc.Dial(proxy, opts...)
	if err != nil {
		return nil, err
	}
	ret := &ProxyConn{
		cc: conn,
	}
	// Make our own copy of these.
	ret.targets = append(ret.targets, targets...)
	return ret, nil
}

// DialContext is the same as Dial except the context provided can be used to cancel or expire the pending connection.
// By default dial operations are non-blocking. See grpc.Dial for a complete explanation.
func DialContext(ctx context.Context, proxy string, targets []string, opts ...grpc.DialOption) (grpc.ClientConnInterface, error) {
	if proxy == "" {
		if len(targets) != 1 {
			return nil, status.Error(codes.InvalidArgument, "no proxy specified but more than one target set")
		}
		return grpc.DialContext(ctx, targets[0], opts...)
	}
	conn, err := grpc.DialContext(ctx, proxy, opts...)
	if err != nil {
		return nil, err
	}
	ret := &ProxyConn{
		cc: conn,
	}
	// Make our own copy of these.
	ret.targets = append(ret.targets, targets...)
	return ret, nil
}
