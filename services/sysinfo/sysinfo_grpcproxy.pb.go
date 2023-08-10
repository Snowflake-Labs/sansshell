// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package sysinfo

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

// SysInfoClientProxy is the superset of SysInfoClient which additionally includes the OneMany proxy methods
type SysInfoClientProxy interface {
	SysInfoClient
	UptimeOneMany(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (<-chan *UptimeManyResponse, error)
	DmesgOneMany(ctx context.Context, in *DmesgRequest, opts ...grpc.CallOption) (SysInfo_DmesgClientProxy, error)
	JournalOneMany(ctx context.Context, in *JournalRequest, opts ...grpc.CallOption) (SysInfo_JournalClientProxy, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type sysInfoClientProxy struct {
	*sysInfoClient
}

// NewSysInfoClientProxy creates a SysInfoClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewSysInfoClientProxy(cc *proxy.Conn) SysInfoClientProxy {
	return &sysInfoClientProxy{NewSysInfoClient(cc).(*sysInfoClient)}
}

// UptimeManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type UptimeManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *UptimeReply
	Error error
}

// UptimeOneMany provides the same API as Uptime but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *sysInfoClientProxy) UptimeOneMany(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (<-chan *UptimeManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *UptimeManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &UptimeManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &UptimeReply{},
			}
			err := conn.Invoke(ctx, "/SysInfo.SysInfo/Uptime", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/SysInfo.SysInfo/Uptime", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &UptimeManyResponse{
				Resp: &UptimeReply{},
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

// DmesgManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type DmesgManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *DmesgReply
	Error error
}

type SysInfo_DmesgClientProxy interface {
	Recv() ([]*DmesgManyResponse, error)
	grpc.ClientStream
}

type sysInfoClientDmesgClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *sysInfoClientDmesgClientProxy) Recv() ([]*DmesgManyResponse, error) {
	var ret []*DmesgManyResponse
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
		m := &DmesgReply{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &DmesgManyResponse{
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
		typedResp := &DmesgManyResponse{
			Resp: &DmesgReply{},
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

// DmesgOneMany provides the same API as Dmesg but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *sysInfoClientProxy) DmesgOneMany(ctx context.Context, in *DmesgRequest, opts ...grpc.CallOption) (SysInfo_DmesgClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &SysInfo_ServiceDesc.Streams[0], "/SysInfo.SysInfo/Dmesg", opts...)
	if err != nil {
		return nil, err
	}
	x := &sysInfoClientDmesgClientProxy{c.cc.(*proxy.Conn), false, stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// JournalManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type JournalManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *JournalReply
	Error error
}

type SysInfo_JournalClientProxy interface {
	Recv() ([]*JournalManyResponse, error)
	grpc.ClientStream
}

type sysInfoClientJournalClientProxy struct {
	cc         *proxy.Conn
	directDone bool
	grpc.ClientStream
}

func (x *sysInfoClientJournalClientProxy) Recv() ([]*JournalManyResponse, error) {
	var ret []*JournalManyResponse
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
		m := &JournalReply{}
		err := x.ClientStream.RecvMsg(m)
		ret = append(ret, &JournalManyResponse{
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
		typedResp := &JournalManyResponse{
			Resp: &JournalReply{},
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

// JournalOneMany provides the same API as Journal but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *sysInfoClientProxy) JournalOneMany(ctx context.Context, in *JournalRequest, opts ...grpc.CallOption) (SysInfo_JournalClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &SysInfo_ServiceDesc.Streams[1], "/SysInfo.SysInfo/Journal", opts...)
	if err != nil {
		return nil, err
	}
	x := &sysInfoClientJournalClientProxy{c.cc.(*proxy.Conn), false, stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}
