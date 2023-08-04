// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package fdb

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

// ConfClientProxy is the superset of ConfClient which additionally includes the OneMany proxy methods
type ConfClientProxy interface {
	ConfClient
	ReadOneMany(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (<-chan *ReadManyResponse, error)
	WriteOneMany(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (<-chan *WriteManyResponse, error)
	DeleteOneMany(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (<-chan *DeleteManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type confClientProxy struct {
	*confClient
}

// NewConfClientProxy creates a ConfClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewConfClientProxy(cc *proxy.Conn) ConfClientProxy {
	return &confClientProxy{NewConfClient(cc).(*confClient)}
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
func (c *confClientProxy) ReadOneMany(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (<-chan *ReadManyResponse, error) {
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
			err := conn.Invoke(ctx, "/Fdb.Conf/Read", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Fdb.Conf/Read", in, opts...)
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
func (c *confClientProxy) WriteOneMany(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (<-chan *WriteManyResponse, error) {
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
			err := conn.Invoke(ctx, "/Fdb.Conf/Write", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Fdb.Conf/Write", in, opts...)
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
func (c *confClientProxy) DeleteOneMany(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (<-chan *DeleteManyResponse, error) {
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
			err := conn.Invoke(ctx, "/Fdb.Conf/Delete", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Fdb.Conf/Delete", in, opts...)
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

// FDBMoveClientProxy is the superset of FDBMoveClient which additionally includes the OneMany proxy methods
type FDBMoveClientProxy interface {
	FDBMoveClient
	FDBMoveDataCopyOneMany(ctx context.Context, in *FDBMoveDataCopyRequest, opts ...grpc.CallOption) (<-chan *FDBMoveDataCopyManyResponse, error)
	FDBMoveDataWaitOneMany(ctx context.Context, in *FDBMoveDataWaitRequest, opts ...grpc.CallOption) (<-chan *FDBMoveDataWaitManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type fDBMoveClientProxy struct {
	*fDBMoveClient
}

// NewFDBMoveClientProxy creates a FDBMoveClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewFDBMoveClientProxy(cc *proxy.Conn) FDBMoveClientProxy {
	return &fDBMoveClientProxy{NewFDBMoveClient(cc).(*fDBMoveClient)}
}

// FDBMoveDataCopyManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type FDBMoveDataCopyManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *FDBMoveDataCopyResponse
	Error error
}

// FDBMoveDataCopyOneMany provides the same API as FDBMoveDataCopy but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *fDBMoveClientProxy) FDBMoveDataCopyOneMany(ctx context.Context, in *FDBMoveDataCopyRequest, opts ...grpc.CallOption) (<-chan *FDBMoveDataCopyManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *FDBMoveDataCopyManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &FDBMoveDataCopyManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &FDBMoveDataCopyResponse{},
			}
			err := conn.Invoke(ctx, "/Fdb.FDBMove/FDBMoveDataCopy", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Fdb.FDBMove/FDBMoveDataCopy", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &FDBMoveDataCopyManyResponse{
				Resp: &FDBMoveDataCopyResponse{},
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

// FDBMoveDataWaitManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type FDBMoveDataWaitManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *FDBMoveDataWaitResponse
	Error error
}

// FDBMoveDataWaitOneMany provides the same API as FDBMoveDataWait but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *fDBMoveClientProxy) FDBMoveDataWaitOneMany(ctx context.Context, in *FDBMoveDataWaitRequest, opts ...grpc.CallOption) (<-chan *FDBMoveDataWaitManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *FDBMoveDataWaitManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &FDBMoveDataWaitManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &FDBMoveDataWaitResponse{},
			}
			err := conn.Invoke(ctx, "/Fdb.FDBMove/FDBMoveDataWait", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Fdb.FDBMove/FDBMoveDataWait", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &FDBMoveDataWaitManyResponse{
				Resp: &FDBMoveDataWaitResponse{},
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

// CLIClientProxy is the superset of CLIClient which additionally includes the OneMany proxy methods
type CLIClientProxy interface {
	CLIClient
	FDBCLIOneMany(ctx context.Context, in *FDBCLIRequest, opts ...grpc.CallOption) (CLI_FDBCLIClientProxy, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type cLIClientProxy struct {
	*cLIClient
}

// NewCLIClientProxy creates a CLIClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewCLIClientProxy(cc *proxy.Conn) CLIClientProxy {
	return &cLIClientProxy{NewCLIClient(cc).(*cLIClient)}
}

// FDBCLIManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type FDBCLIManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *FDBCLIResponse
	Error error
}

type CLI_FDBCLIClientProxy interface {
	Recv() ([]*FDBCLIManyResponse, error)
	grpc.ClientStream
}

type cLIClientFDBCLIClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *cLIClientFDBCLIClientProxy) Recv() ([]*FDBCLIManyResponse, error) {
	var ret []*FDBCLIManyResponse
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
		m := &FDBCLIResponse{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &FDBCLIManyResponse{
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
		typedResp := &FDBCLIManyResponse{
			Resp: &FDBCLIResponse{},
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

// FDBCLIOneMany provides the same API as FDBCLI but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *cLIClientProxy) FDBCLIOneMany(ctx context.Context, in *FDBCLIRequest, opts ...grpc.CallOption) (CLI_FDBCLIClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &CLI_ServiceDesc.Streams[0], "/Fdb.CLI/FDBCLI", opts...)
	if err != nil {
		return nil, err
	}
	x := &cLIClientFDBCLIClientProxy{c.cc.(*proxy.Conn), false, stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// ServerClientProxy is the superset of ServerClient which additionally includes the OneMany proxy methods
type ServerClientProxy interface {
	ServerClient
	FDBServerOneMany(ctx context.Context, in *FDBServerRequest, opts ...grpc.CallOption) (<-chan *FDBServerManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type serverClientProxy struct {
	*serverClient
}

// NewServerClientProxy creates a ServerClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewServerClientProxy(cc *proxy.Conn) ServerClientProxy {
	return &serverClientProxy{NewServerClient(cc).(*serverClient)}
}

// FDBServerManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type FDBServerManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *FDBServerResponse
	Error error
}

// FDBServerOneMany provides the same API as FDBServer but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *serverClientProxy) FDBServerOneMany(ctx context.Context, in *FDBServerRequest, opts ...grpc.CallOption) (<-chan *FDBServerManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *FDBServerManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &FDBServerManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &FDBServerResponse{},
			}
			err := conn.Invoke(ctx, "/Fdb.Server/FDBServer", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Fdb.Server/FDBServer", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &FDBServerManyResponse{
				Resp: &FDBServerResponse{},
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
