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
	"io"
)

// LocalFileClientProxy is the superset of LocalFileClient which additionally includes the OneMany proxy methods
type LocalFileClientProxy interface {
	LocalFileClient
	ReadOneMany(ctx context.Context, in *ReadActionRequest, opts ...grpc.CallOption) (LocalFile_ReadClientProxy, error)
	StatOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_StatClientProxy, error)
	SumOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_SumClientProxy, error)
	WriteOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_WriteClientProxy, error)
	CopyOneMany(ctx context.Context, in *CopyRequest, opts ...grpc.CallOption) (<-chan *CopyManyResponse, error)
	ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (LocalFile_ListClientProxy, error)
	SetFileAttributesOneMany(ctx context.Context, in *SetFileAttributesRequest, opts ...grpc.CallOption) (<-chan *SetFileAttributesManyResponse, error)
	RmOneMany(ctx context.Context, in *RmRequest, opts ...grpc.CallOption) (<-chan *RmManyResponse, error)
	RmdirOneMany(ctx context.Context, in *RmdirRequest, opts ...grpc.CallOption) (<-chan *RmdirManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type localFileClientProxy struct {
	*localFileClient
}

// NewLocalFileClientProxy creates a LocalFileClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewLocalFileClientProxy(cc *proxy.Conn) LocalFileClientProxy {
	return &localFileClientProxy{NewLocalFileClient(cc).(*localFileClient)}
}

// ReadManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type ReadManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *ReadReply
	Error error
}

type LocalFile_ReadClientProxy interface {
	Recv() ([]*ReadManyResponse, error)
	grpc.ClientStream
}

type localFileClientReadClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *localFileClientReadClientProxy) Recv() ([]*ReadManyResponse, error) {
	var ret []*ReadManyResponse
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
		m := &ReadReply{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &ReadManyResponse{
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
		typedResp := &ReadManyResponse{
			Resp: &ReadReply{},
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

// ReadOneMany provides the same API as Read but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) ReadOneMany(ctx context.Context, in *ReadActionRequest, opts ...grpc.CallOption) (LocalFile_ReadClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[0], "/LocalFile.LocalFile/Read", opts...)
	if err != nil {
		return nil, err
	}
	x := &localFileClientReadClientProxy{c.cc.(*proxy.Conn), false, stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// StatManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type StatManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
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
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *localFileClientStatClientProxy) Send(m *StatRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *localFileClientStatClientProxy) Recv() ([]*StatManyResponse, error) {
	var ret []*StatManyResponse
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
		m := &StatReply{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &StatManyResponse{
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
		typedResp := &StatManyResponse{
			Resp: &StatReply{},
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

// StatOneMany provides the same API as Stat but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) StatOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_StatClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[1], "/LocalFile.LocalFile/Stat", opts...)
	if err != nil {
		return nil, err
	}
	x := &localFileClientStatClientProxy{c.cc.(*proxy.Conn), false, stream}
	return x, nil
}

// SumManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type SumManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
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
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *localFileClientSumClientProxy) Send(m *SumRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *localFileClientSumClientProxy) Recv() ([]*SumManyResponse, error) {
	var ret []*SumManyResponse
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
		m := &SumReply{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &SumManyResponse{
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
		typedResp := &SumManyResponse{
			Resp: &SumReply{},
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

// SumOneMany provides the same API as Sum but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) SumOneMany(ctx context.Context, opts ...grpc.CallOption) (LocalFile_SumClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[2], "/LocalFile.LocalFile/Sum", opts...)
	if err != nil {
		return nil, err
	}
	x := &localFileClientSumClientProxy{c.cc.(*proxy.Conn), false, stream}
	return x, nil
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

type LocalFile_WriteClientProxy interface {
	Send(*WriteRequest) error
	CloseAndRecv() ([]*WriteManyResponse, error)
	grpc.ClientStream
}

type localFileClientWriteClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *localFileClientWriteClientProxy) Send(m *WriteRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *localFileClientWriteClientProxy) CloseAndRecv() ([]*WriteManyResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	var ret []*WriteManyResponse
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
		m := &emptypb.Empty{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &WriteManyResponse{
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

	eof := make(map[int]bool)
	for i := range x.cc.Targets {
		eof[i] = false
	}
	for {
		// Need to allow all client channels to return state before we return since
		// no more Recv's will ever be called.
		done := true
		for _, v := range eof {
			if !v {
				done = false
			}
		}
		if done {
			break
		}
		m := []*proxy.Ret{}
		if err := x.ClientStream.RecvMsg(&m); err != nil {
			return nil, err
		}
		for _, r := range m {
			typedResp := &WriteManyResponse{
				Resp: &emptypb.Empty{},
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
			eof[r.Index] = true
		}
	}
	return ret, nil
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
	x := &localFileClientWriteClientProxy{c.cc.(*proxy.Conn), false, stream}
	return x, nil
}

// CopyManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type CopyManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

// CopyOneMany provides the same API as Copy but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) CopyOneMany(ctx context.Context, in *CopyRequest, opts ...grpc.CallOption) (<-chan *CopyManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *CopyManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
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

// ListManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type ListManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *ListReply
	Error error
}

type LocalFile_ListClientProxy interface {
	Recv() ([]*ListManyResponse, error)
	grpc.ClientStream
}

type localFileClientListClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *localFileClientListClientProxy) Recv() ([]*ListManyResponse, error) {
	var ret []*ListManyResponse
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
		m := &ListReply{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &ListManyResponse{
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
		typedResp := &ListManyResponse{
			Resp: &ListReply{},
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

// ListOneMany provides the same API as List but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (LocalFile_ListClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &LocalFile_ServiceDesc.Streams[4], "/LocalFile.LocalFile/List", opts...)
	if err != nil {
		return nil, err
	}
	x := &localFileClientListClientProxy{c.cc.(*proxy.Conn), false, stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// SetFileAttributesManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type SetFileAttributesManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

// SetFileAttributesOneMany provides the same API as SetFileAttributes but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) SetFileAttributesOneMany(ctx context.Context, in *SetFileAttributesRequest, opts ...grpc.CallOption) (<-chan *SetFileAttributesManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *SetFileAttributesManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
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

// RmManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type RmManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

// RmOneMany provides the same API as Rm but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) RmOneMany(ctx context.Context, in *RmRequest, opts ...grpc.CallOption) (<-chan *RmManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *RmManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &RmManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &emptypb.Empty{},
			}
			err := conn.Invoke(ctx, "/LocalFile.LocalFile/Rm", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/LocalFile.LocalFile/Rm", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &RmManyResponse{
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

// RmdirManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type RmdirManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *emptypb.Empty
	Error error
}

// RmdirOneMany provides the same API as Rmdir but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *localFileClientProxy) RmdirOneMany(ctx context.Context, in *RmdirRequest, opts ...grpc.CallOption) (<-chan *RmdirManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *RmdirManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &RmdirManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &emptypb.Empty{},
			}
			err := conn.Invoke(ctx, "/LocalFile.LocalFile/Rmdir", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/LocalFile.LocalFile/Rmdir", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &RmdirManyResponse{
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
