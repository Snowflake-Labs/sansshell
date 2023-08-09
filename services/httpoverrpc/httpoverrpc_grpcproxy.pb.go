// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package httpoverrpc

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"fmt"
)

// HTTPOverRPCClientProxy is the superset of HTTPOverRPCClient which additionally includes the OneMany proxy methods
type HTTPOverRPCClientProxy interface {
	HTTPOverRPCClient
	HostOneMany(ctx context.Context, in *HostHTTPRequest, opts ...grpc.CallOption) (<-chan *HostManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type hTTPOverRPCClientProxy struct {
	*hTTPOverRPCClient
}

// NewHTTPOverRPCClientProxy creates a HTTPOverRPCClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewHTTPOverRPCClientProxy(cc *proxy.Conn) HTTPOverRPCClientProxy {
	return &hTTPOverRPCClientProxy{NewHTTPOverRPCClient(cc).(*hTTPOverRPCClient)}
}

// HostManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type HostManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *HTTPReply
	Error error
}

// HostOneMany provides the same API as Host but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *hTTPOverRPCClientProxy) HostOneMany(ctx context.Context, in *HostHTTPRequest, opts ...grpc.CallOption) (<-chan *HostManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *HostManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &HostManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &HTTPReply{},
			}
			err := conn.Invoke(ctx, "/HTTPOverRPC.HTTPOverRPC/Host", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/HTTPOverRPC.HTTPOverRPC/Host", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &HostManyResponse{
				Resp: &HTTPReply{},
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
