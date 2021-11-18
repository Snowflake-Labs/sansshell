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
	"fmt"
)

// ProcessClientProxy is the superset of ProcessClient which additionally includes the OneMany proxy methods
type ProcessClientProxy interface {
	ProcessClient
	ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (<-chan *ListManyResponse, error)
	GetStacksOneMany(ctx context.Context, in *GetStacksRequest, opts ...grpc.CallOption) (<-chan *GetStacksManyResponse, error)
	GetJavaStacksOneMany(ctx context.Context, in *GetJavaStacksRequest, opts ...grpc.CallOption) (<-chan *GetJavaStacksManyResponse, error)
	GetMemoryDumpOneMany(ctx context.Context, in *GetMemoryDumpRequest, opts ...grpc.CallOption) (Process_GetMemoryDumpClientProxy, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
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
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *ListReply
	Error error
}

// ListOneMany provides the same API as List but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *processClientProxy) ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (<-chan *ListManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *ListManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
	if conn.NumTargets() == 1 {
		go func() {
			out := &ListManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ListReply{},
			}
			err := conn.Invoke(ctx, "/Process.Process/List", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Process.Process/List", in, opts...)
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

type GetStacksManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *GetStacksReply
	Error error
}

// GetStacksOneMany provides the same API as GetStacks but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *processClientProxy) GetStacksOneMany(ctx context.Context, in *GetStacksRequest, opts ...grpc.CallOption) (<-chan *GetStacksManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *GetStacksManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
	if conn.NumTargets() == 1 {
		go func() {
			out := &GetStacksManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &GetStacksReply{},
			}
			err := conn.Invoke(ctx, "/Process.Process/GetStacks", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Process.Process/GetStacks", in, opts...)
	if err != nil {
		return nil, err
	}
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

type GetJavaStacksManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *GetJavaStacksReply
	Error error
}

// GetJavaStacksOneMany provides the same API as GetJavaStacks but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *processClientProxy) GetJavaStacksOneMany(ctx context.Context, in *GetJavaStacksRequest, opts ...grpc.CallOption) (<-chan *GetJavaStacksManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *GetJavaStacksManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
	if conn.NumTargets() == 1 {
		go func() {
			out := &GetJavaStacksManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &GetJavaStacksReply{},
			}
			err := conn.Invoke(ctx, "/Process.Process/GetJavaStacks", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Process.Process/GetJavaStacks", in, opts...)
	if err != nil {
		return nil, err
	}
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

type GetMemoryDumpManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *GetMemoryDumpReply
	Error error
}

type Process_GetMemoryDumpClientProxy interface {
	Recv() ([]*GetMemoryDumpManyResponse, error)
	grpc.ClientStream
}

type processClientGetMemoryDumpClientProxy struct {
	grpc.ClientStream
}

func (x *processClientGetMemoryDumpClientProxy) Recv() ([]*GetMemoryDumpManyResponse, error) {
	ret := []*GetMemoryDumpManyResponse{}
	m := []*proxy.ProxyRet{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	for _, r := range m {
		typedResp := &GetMemoryDumpManyResponse{
			Resp: &GetMemoryDumpReply{},
		}
		typedResp.Target = r.Target
		typedResp.Index = r.Index
		typedResp.Error = r.Error
		if r.Error == nil {
			if err := r.Resp.UnmarshalTo(typedResp.Resp); err != nil {
				typedResp.Error = fmt.Errorf("can't decode any response - %v. Original Error - %v", err, r.Error)
			}
		}
		ret = append(ret, typedResp)
	}
	return ret, nil
}

// GetMemoryDumpOneMany provides the same API as GetMemoryDump but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *processClientProxy) GetMemoryDumpOneMany(ctx context.Context, in *GetMemoryDumpRequest, opts ...grpc.CallOption) (Process_GetMemoryDumpClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &Process_ServiceDesc.Streams[0], "/Process.Process/GetMemoryDump", opts...)
	if err != nil {
		return nil, err
	}
	x := &processClientGetMemoryDumpClientProxy{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}
