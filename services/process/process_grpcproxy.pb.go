// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package process

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"errors"
	"fmt"
)

// ProcessClientProxy is the superset of ProcessClient which additionally includes the OneMany proxy methods
type ProcessClientProxy interface {
	ProcessClient
	ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (<-chan *ListManyResponse, error)
	GetStacksOneMany(ctx context.Context, in *GetStacksRequest, opts ...grpc.CallOption) (<-chan *GetStacksManyResponse, error)
	GetJavaStacksOneMany(ctx context.Context, in *GetJavaStacksRequest, opts ...grpc.CallOption) (<-chan *GetJavaStacksManyResponse, error)
	GetMemoryDumpOneMany(ctx context.Context, in *GetMemoryDumpRequest, opts ...grpc.CallOption) (<-chan *GetMemoryDumpManyResponse, error)
}

type processClientProxy struct {
	*processClient
}

// NewProcessClientProxy creates a ProcessClientProxy for use in proxied connections.
// NOTE: This takes a ProxyConn instead of a generic ClientConnInterface as the methods here are only valid in ProxyConn contexts.
func NewProcessClientProxy(cc *proxy.ProxyConn) ProcessClientProxy {
	return &processClientProxy{NewProcessClient(cc).(*processClient)}
}

type ListManyResponse struct {
	Target string
	Resp   *ListReply
	Error  error
}

func (c *processClientProxy) ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (<-chan *ListManyResponse, error) {
	manyRet, err := c.cc.(*proxy.ProxyConn).InvokeOneMany(ctx, "/Process.Process/List", in, opts...)
	if err != nil {
		return nil, err
	}
	ret := make(chan *ListManyResponse)
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

type GetStacksManyResponse struct {
	Target string
	Resp   *GetStacksReply
	Error  error
}

func (c *processClientProxy) GetStacksOneMany(ctx context.Context, in *GetStacksRequest, opts ...grpc.CallOption) (<-chan *GetStacksManyResponse, error) {
	manyRet, err := c.cc.(*proxy.ProxyConn).InvokeOneMany(ctx, "/Process.Process/GetStacks", in, opts...)
	if err != nil {
		return nil, err
	}
	ret := make(chan *GetStacksManyResponse)
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &GetStacksManyResponse{
				Resp: &GetStacksReply{},
			}

			resp, ok := <-manyRet
			if !ok {
				// All done so we can shut down.
				close(ret)
				return
			}
			typedResp.Target = resp.Target
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

type GetJavaStacksManyResponse struct {
	Target string
	Resp   *GetJavaStacksReply
	Error  error
}

func (c *processClientProxy) GetJavaStacksOneMany(ctx context.Context, in *GetJavaStacksRequest, opts ...grpc.CallOption) (<-chan *GetJavaStacksManyResponse, error) {
	manyRet, err := c.cc.(*proxy.ProxyConn).InvokeOneMany(ctx, "/Process.Process/GetJavaStacks", in, opts...)
	if err != nil {
		return nil, err
	}
	ret := make(chan *GetJavaStacksManyResponse)
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &GetJavaStacksManyResponse{
				Resp: &GetJavaStacksReply{},
			}

			resp, ok := <-manyRet
			if !ok {
				// All done so we can shut down.
				close(ret)
				return
			}
			typedResp.Target = resp.Target
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

type GetMemoryDumpManyResponse struct {
	Target string
	Resp   *GetMemoryDumpReply
	Error  error
}

func (c *processClientProxy) GetMemoryDumpOneMany(ctx context.Context, in *GetMemoryDumpRequest, opts ...grpc.CallOption) (<-chan *GetMemoryDumpManyResponse, error) {
	return nil, errors.New("not implemented")
}
