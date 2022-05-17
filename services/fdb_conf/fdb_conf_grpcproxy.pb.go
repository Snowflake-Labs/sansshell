// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package fdb_conf

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

import (
	"fmt"
)

// FdbConfClientProxy is the superset of FdbConfClient which additionally includes the OneMany proxy methods
type FdbConfClientProxy interface {
	FdbConfClient
	ReadOneMany(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (<-chan *ReadManyResponse, error)
	WriteOneMany(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (<-chan *WriteManyResponse, error)
	DeleteOneMany(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (<-chan *DeleteManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type fdbConfClientProxy struct {
	*fdbConfClient
}

// NewFdbConfClientProxy creates a FdbConfClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewFdbConfClientProxy(cc *proxy.Conn) FdbConfClientProxy {
	return &fdbConfClientProxy{NewFdbConfClient(cc).(*fdbConfClient)}
}

// ReadManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type ReadManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *FdbConfResponse
	Error error
}

// ReadOneMany provides the same API as Read but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *fdbConfClientProxy) ReadOneMany(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (<-chan *ReadManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *ReadManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ReadManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &FdbConfResponse{},
			}
			err := conn.Invoke(ctx, "/FdbConf.FdbConf/Read", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/FdbConf.FdbConf/Read", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ReadManyResponse{
				Resp: &FdbConfResponse{},
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

// WriteManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type WriteManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

// WriteOneMany provides the same API as Write but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *fdbConfClientProxy) WriteOneMany(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (<-chan *WriteManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *WriteManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &WriteManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &emptypb.Empty{},
			}
			err := conn.Invoke(ctx, "/FdbConf.FdbConf/Write", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/FdbConf.FdbConf/Write", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &WriteManyResponse{
				Resp: &emptypb.Empty{},
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

// DeleteManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type DeleteManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

// DeleteOneMany provides the same API as Delete but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *fdbConfClientProxy) DeleteOneMany(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (<-chan *DeleteManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *DeleteManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &DeleteManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &emptypb.Empty{},
			}
			err := conn.Invoke(ctx, "/FdbConf.FdbConf/Delete", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/FdbConf.FdbConf/Delete", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &DeleteManyResponse{
				Resp: &emptypb.Empty{},
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
