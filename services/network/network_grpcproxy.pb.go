// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package network

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

import (
	"fmt"
	"io"
)

// PacketCaptureClientProxy is the superset of PacketCaptureClient which additionally includes the OneMany proxy methods
type PacketCaptureClientProxy interface {
	PacketCaptureClient
	RawStreamOneMany(ctx context.Context, in *RawStreamRequest, opts ...grpc.CallOption) (PacketCapture_RawStreamClientProxy, error)
	ListInterfacesOneMany(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (<-chan *ListInterfacesManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type packetCaptureClientProxy struct {
	*packetCaptureClient
}

// NewPacketCaptureClientProxy creates a PacketCaptureClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewPacketCaptureClientProxy(cc *proxy.Conn) PacketCaptureClientProxy {
	return &packetCaptureClientProxy{NewPacketCaptureClient(cc).(*packetCaptureClient)}
}

// RawStreamManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type RawStreamManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *RawStreamReply
	Error error
}

type PacketCapture_RawStreamClientProxy interface {
	Recv() ([]*RawStreamManyResponse, error)
	grpc.ClientStream
}

type packetCaptureClientRawStreamClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *packetCaptureClientRawStreamClientProxy) Recv() ([]*RawStreamManyResponse, error) {
	var ret []*RawStreamManyResponse
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
		m := &RawStreamReply{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &RawStreamManyResponse{
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
		typedResp := &RawStreamManyResponse{
			Resp: &RawStreamReply{},
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

// RawStreamOneMany provides the same API as RawStream but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packetCaptureClientProxy) RawStreamOneMany(ctx context.Context, in *RawStreamRequest, opts ...grpc.CallOption) (PacketCapture_RawStreamClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &PacketCapture_ServiceDesc.Streams[0], "/Network.PacketCapture/RawStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &packetCaptureClientRawStreamClientProxy{c.cc.(*proxy.Conn), false, stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// ListInterfacesManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type ListInterfacesManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *ListInterfacesReply
	Error error
}

// ListInterfacesOneMany provides the same API as ListInterfaces but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packetCaptureClientProxy) ListInterfacesOneMany(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (<-chan *ListInterfacesManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *ListInterfacesManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ListInterfacesManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ListInterfacesReply{},
			}
			err := conn.Invoke(ctx, "/Network.PacketCapture/ListInterfaces", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Network.PacketCapture/ListInterfaces", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ListInterfacesManyResponse{
				Resp: &ListInterfacesReply{},
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
