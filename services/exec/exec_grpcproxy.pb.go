// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package exec

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"fmt"
	"io"
)

// ExecClientProxy is the superset of ExecClient which additionally includes the OneMany proxy methods
type ExecClientProxy interface {
	ExecClient
	RunOneMany(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (<-chan *RunManyResponse, error)
	StreamingRunOneMany(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (Exec_StreamingRunClientProxy, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type execClientProxy struct {
	*execClient
}

// NewExecClientProxy creates a ExecClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewExecClientProxy(cc *proxy.Conn) ExecClientProxy {
	return &execClientProxy{NewExecClient(cc).(*execClient)}
}

// RunManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type RunManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *ExecResponse
	Error error
}

// RunOneMany provides the same API as Run but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *execClientProxy) RunOneMany(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (<-chan *RunManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *RunManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &RunManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ExecResponse{},
			}
			err := conn.Invoke(ctx, "/Exec.Exec/Run", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Exec.Exec/Run", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &RunManyResponse{
				Resp: &ExecResponse{},
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

// StreamingRunManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type StreamingRunManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *ExecResponse
	Error error
}

type Exec_StreamingRunClientProxy interface {
	Recv() ([]*StreamingRunManyResponse, error)
	grpc.ClientStream
}

type execClientStreamingRunClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *execClientStreamingRunClientProxy) Recv() ([]*StreamingRunManyResponse, error) {
	var ret []*StreamingRunManyResponse
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
		m := &ExecResponse{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &StreamingRunManyResponse{
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
		typedResp := &StreamingRunManyResponse{
			Resp: &ExecResponse{},
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

// StreamingRunOneMany provides the same API as StreamingRun but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *execClientProxy) StreamingRunOneMany(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (Exec_StreamingRunClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &Exec_ServiceDesc.Streams[0], "/Exec.Exec/StreamingRun", opts...)
	if err != nil {
		return nil, err
	}
	x := &execClientStreamingRunClientProxy{c.cc.(*proxy.Conn), false, stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}
