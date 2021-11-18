// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package localfile

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

import (
	"fmt"
)

// LocalFileClientProxy is the superset of LocalFileClient which additionally includes the OneMany proxy methods
type LocalFileClientProxy interface {
	LocalFileClient
	ReadOneMany(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (LocalFile_ReadClientProxy, error)
	StatOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_StatClientProxy, error)
	SumOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_SumClientProxy, error)
	WriteOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_WriteClientProxy, error)
	CopyOneMany(ctx context.Context, in *CopyRequest, opts ...grpc.CallOption) (<-chan *CopyManyResponse, error)
	SetFileAttributesOneMany(ctx context.Context, in *SetFileAttributesRequest, opts ...grpc.CallOption) (<-chan *SetFileAttributesManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type localFileClientProxy struct {
	*localFileClient
}

// NewLocalFileClientProxy creates a LocalFileClientProxy for use in proxied connections.
// NOTE: This takes a ProxyConn instead of a generic ClientConnInterface as the methods here are only valid in ProxyConn contexts.
func NewLocalFileClientProxy(cc *proxy.ProxyConn) LocalFileClientProxy {
	return &localFileClientProxy{NewLocalFileClient(cc).(*localFileClient)}
}

type ReadManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *ReadReply
	Error error
}

type LocalFile_ReadClientProxy interface {
	Recv() ([]*ReadManyResponse, error)
	grpc.ClientStream
}

type localFileClientReadClientProxy struct {
	grpc.ClientStream
}

func (x *localFileClientReadClientProxy) Recv() ([]*ReadManyResponse, error) {
	m := []*ReadManyResponse{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// ReadOneMany provides the same API as Read but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) ReadOneMany(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (LocalFile_ReadClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[0], "/LocalFile.LocalFile/Read", opts...)
	if err != nil {
		return nil, err
	}
	x := &localFileClientReadClientProxy{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type StatManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *StatReply
	Error error
}

type LocalFile_StatClientProxy interface {
	Send(*StatRequest) error
	Recv() ([]*StatManyResponse, error)
	grpc.ClientStream
}

type localFileClientStatClientProxy struct {
	grpc.ClientStream
}

func (x *localFileClientStatClientProxy) Send(m *StatRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *localFileClientStatClientProxy) Recv() ([]*StatManyResponse, error) {
	m := []*StatManyResponse{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// StatOneMany provides the same API as Stat but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) StatOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_StatClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[1], "/LocalFile.LocalFile/Stat", opts...)
	if err != nil {
		return nil, err
	}
	x := &localFileClientStatClientProxy{stream}
	return x, nil
}

type SumManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *SumReply
	Error error
}

type LocalFile_SumClientProxy interface {
	Send(*SumRequest) error
	Recv() ([]*SumManyResponse, error)
	grpc.ClientStream
}

type localFileClientSumClientProxy struct {
	grpc.ClientStream
}

func (x *localFileClientSumClientProxy) Send(m *SumRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *localFileClientSumClientProxy) Recv() ([]*SumManyResponse, error) {
	m := []*SumManyResponse{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// SumOneMany provides the same API as Sum but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) SumOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_SumClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[2], "/LocalFile.LocalFile/Sum", opts...)
	if err != nil {
		return nil, err
	}
	x := &localFileClientSumClientProxy{stream}
	return x, nil
}

type WriteManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

type LocalFile_WriteClientProxy interface {
	Send(*WriteRequest) error
	CloseAndRecv() (*emptypb.Empty, error)
	grpc.ClientStream
}

type localFileClientWriteClientProxy struct {
	grpc.ClientStream
}

func (x *localFileClientWriteClientProxy) Send(m *WriteRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *localFileClientWriteClientProxy) CloseAndRecv() (*emptypb.Empty, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(emptypb.Empty)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// WriteOneMany provides the same API as Write but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) WriteOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_WriteClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[3], "/LocalFile.LocalFile/Write", opts...)
	if err != nil {
		return nil, err
	}
	x := &localFileClientWriteClientProxy{stream}
	return x, nil
}

type CopyManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

// CopyOneMany provides the same API as Copy but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) CopyOneMany(ctx context.Context, in *CopyRequest, opts ...grpc.CallOption) (<-chan *CopyManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *CopyManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
	if conn.NumTargets() == 1 {
		go func() {
			out := &CopyManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &emptypb.Empty{},
			}
			err := conn.Invoke(ctx, "/LocalFile.LocalFile/Copy", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/LocalFile.LocalFile/Copy", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &CopyManyResponse{
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

type SetFileAttributesManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

// SetFileAttributesOneMany provides the same API as SetFileAttributes but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) SetFileAttributesOneMany(ctx context.Context, in *SetFileAttributesRequest, opts ...grpc.CallOption) (<-chan *SetFileAttributesManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *SetFileAttributesManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
	if conn.NumTargets() == 1 {
		go func() {
			out := &SetFileAttributesManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &emptypb.Empty{},
			}
			err := conn.Invoke(ctx, "/LocalFile.LocalFile/SetFileAttributes", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/LocalFile.LocalFile/SetFileAttributes", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &SetFileAttributesManyResponse{
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
