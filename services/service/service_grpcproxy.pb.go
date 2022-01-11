// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package service

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"fmt"
)

// ServiceClientProxy is the superset of ServiceClient which additionally includes the OneMany proxy methods
type ServiceClientProxy interface {
	ServiceClient
	ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (<-chan *ListManyResponse, error)
	StatusOneMany(ctx context.Context, in *StatusRequest, opts ...grpc.CallOption) (<-chan *StatusManyResponse, error)
	ActionOneMany(ctx context.Context, in *ActionRequest, opts ...grpc.CallOption) (<-chan *ActionManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type serviceClientProxy struct {
	*serviceClient
}

// NewServiceClientProxy creates a ServiceClientProxy for use in proxied connections.
// NOTE: This takes a ProxyConn instead of a generic ClientConnInterface as the methods here are only valid in ProxyConn contexts.
func NewServiceClientProxy(cc *proxy.ProxyConn) ServiceClientProxy {
	return &serviceClientProxy{NewServiceClient(cc).(*serviceClient)}
}

type ListManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *ListReply
	Error error
}

// ListOneMany provides the same API as List but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *serviceClientProxy) ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (<-chan *ListManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *ListManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ListManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ListReply{},
			}
			err := conn.Invoke(ctx, "/Service.Service/List", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Service.Service/List", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ListManyResponse{
				Resp: &ListReply{},
			}

			resp, ok := <-manyRet
			if !ok {
				// All done so we can shut down.
				close(ret)
				return
			}
			typedResp.Target = resp.Target
			typedResp.Index = resp.Index
			typedResp.Error = resp.Error
			if resp.Error == nil {
				if err := resp.Resp.UnmarshalTo(typedResp.Resp); err != nil {
					typedResp.Error = fmt.Errorf("can't decode any response - %v. Original Error - %v", err, resp.Error)
				}
			}
			ret <- typedResp
		}
	}()

	return ret, nil
}

type StatusManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *StatusReply
	Error error
}

// StatusOneMany provides the same API as Status but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *serviceClientProxy) StatusOneMany(ctx context.Context, in *StatusRequest, opts ...grpc.CallOption) (<-chan *StatusManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *StatusManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &StatusManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &StatusReply{},
			}
			err := conn.Invoke(ctx, "/Service.Service/Status", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Service.Service/Status", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &StatusManyResponse{
				Resp: &StatusReply{},
			}

			resp, ok := <-manyRet
			if !ok {
				// All done so we can shut down.
				close(ret)
				return
			}
			typedResp.Target = resp.Target
			typedResp.Index = resp.Index
			typedResp.Error = resp.Error
			if resp.Error == nil {
				if err := resp.Resp.UnmarshalTo(typedResp.Resp); err != nil {
					typedResp.Error = fmt.Errorf("can't decode any response - %v. Original Error - %v", err, resp.Error)
				}
			}
			ret <- typedResp
		}
	}()

	return ret, nil
}

type ActionManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *ActionReply
	Error error
}

// ActionOneMany provides the same API as Action but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *serviceClientProxy) ActionOneMany(ctx context.Context, in *ActionRequest, opts ...grpc.CallOption) (<-chan *ActionManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *ActionManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ActionManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ActionReply{},
			}
			err := conn.Invoke(ctx, "/Service.Service/Action", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Service.Service/Action", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ActionManyResponse{
				Resp: &ActionReply{},
			}

			resp, ok := <-manyRet
			if !ok {
				// All done so we can shut down.
				close(ret)
				return
			}
			typedResp.Target = resp.Target
			typedResp.Index = resp.Index
			typedResp.Error = resp.Error
			if resp.Error == nil {
				if err := resp.Resp.UnmarshalTo(typedResp.Resp); err != nil {
					typedResp.Error = fmt.Errorf("can't decode any response - %v. Original Error - %v", err, resp.Error)
				}
			}
			ret <- typedResp
		}
	}()

	return ret, nil
}
