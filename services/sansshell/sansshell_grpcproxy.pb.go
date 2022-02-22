// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package sansshell

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

import (
	"fmt"
)

// LoggingClientProxy is the superset of LoggingClient which additionally includes the OneMany proxy methods
type LoggingClientProxy interface {
	LoggingClient
	SetVerbosityOneMany(ctx context.Context, in *SetVerbosityRequest, opts ...grpc.CallOption) (<-chan *SetVerbosityManyResponse, error)
	GetVerbosityOneMany(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (<-chan *GetVerbosityManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type loggingClientProxy struct {
	*loggingClient
}

// NewLoggingClientProxy creates a LoggingClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewLoggingClientProxy(cc *proxy.Conn) LoggingClientProxy {
	return &loggingClientProxy{NewLoggingClient(cc).(*loggingClient)}
}

// SetVerbosityManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type SetVerbosityManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *VerbosityReply
	Error error
}

// SetVerbosityOneMany provides the same API as SetVerbosity but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *loggingClientProxy) SetVerbosityOneMany(ctx context.Context, in *SetVerbosityRequest, opts ...grpc.CallOption) (<-chan *SetVerbosityManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *SetVerbosityManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &SetVerbosityManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &VerbosityReply{},
			}
			err := conn.Invoke(ctx, "/Sansshell.Logging/SetVerbosity", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Sansshell.Logging/SetVerbosity", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &SetVerbosityManyResponse{
				Resp: &VerbosityReply{},
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

// GetVerbosityManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type GetVerbosityManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *VerbosityReply
	Error error
}

// GetVerbosityOneMany provides the same API as GetVerbosity but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *loggingClientProxy) GetVerbosityOneMany(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (<-chan *GetVerbosityManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *GetVerbosityManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &GetVerbosityManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &VerbosityReply{},
			}
			err := conn.Invoke(ctx, "/Sansshell.Logging/GetVerbosity", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Sansshell.Logging/GetVerbosity", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &GetVerbosityManyResponse{
				Resp: &VerbosityReply{},
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
