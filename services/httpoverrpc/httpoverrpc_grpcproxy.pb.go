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
	"io"
)

// HTTPOverRPCClientProxy is the superset of HTTPOverRPCClient which additionally includes the OneMany proxy methods
type HTTPOverRPCClientProxy interface {
	HTTPOverRPCClient
	HostOneMany(ctx context.Context, in *HostHTTPRequest, opts ...grpc.CallOption) (<-chan *HostManyResponse, error)
	StreamHostOneMany(ctx context.Context, in *HostHTTPRequest, opts ...grpc.CallOption) (HTTPOverRPC_StreamHostClientProxy, error)
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

// StreamHostManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type StreamHostManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *HTTPStreamReply
	Error error
}

type HTTPOverRPC_StreamHostClientProxy interface {
	Recv() ([]*StreamHostManyResponse, error)
	grpc.ClientStream
}

type hTTPOverRPCClientStreamHostClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *hTTPOverRPCClientStreamHostClientProxy) Recv() ([]*StreamHostManyResponse, error) {
	var ret []*StreamHostManyResponse
	// If this is a direct connection the RecvMsg call is to a standard grpc.ClientStream
	// and not our proxy based one. This means we need to receive a typed response and
	// convert it into a single slice entry return. This ensures the OneMany style calls
	// can be used by proxy with 1:N targets and non proxy with 1 target without client changes.
	if x.cc.Direct() {
		// Check if we're done. Just return EOF now. Any real error was already sent inside
		// of a ManyResponse.
		if x.directDone {
			return nil, io.EOF
		}
		m := &HTTPStreamReply{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &StreamHostManyResponse{
			Resp:   m,
			Error:  err,
			Target: x.cc.Targets[0],
			Index:  0,
		})
		// An error means we're done so set things so a later call now gets an EOF.
		if err != nil {
			x.directDone = true
		}
		return ret, nil
	}

	m := []*proxy.Ret{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	for _, r := range m {
		typedResp := &StreamHostManyResponse{
			Resp: &HTTPStreamReply{},
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

// StreamHostOneMany provides the same API as StreamHost but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *hTTPOverRPCClientProxy) StreamHostOneMany(ctx context.Context, in *HostHTTPRequest, opts ...grpc.CallOption) (HTTPOverRPC_StreamHostClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &HTTPOverRPC_ServiceDesc.Streams[0], "/HTTPOverRPC.HTTPOverRPC/StreamHost", opts...)
	if err != nil {
		return nil, err
	}
	x := &hTTPOverRPCClientStreamHostClientProxy{c.cc.(*proxy.Conn), false, stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}
