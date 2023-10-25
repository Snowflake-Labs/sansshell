// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package mpa

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"fmt"
)

// MpaClientProxy is the superset of MpaClient which additionally includes the OneMany proxy methods
type MpaClientProxy interface {
	MpaClient
	StoreOneMany(ctx context.Context, in *StoreRequest, opts ...grpc.CallOption) (<-chan *StoreManyResponse, error)
	ApproveOneMany(ctx context.Context, in *ApproveRequest, opts ...grpc.CallOption) (<-chan *ApproveManyResponse, error)
	WaitForApprovalOneMany(ctx context.Context, in *WaitForApprovalRequest, opts ...grpc.CallOption) (<-chan *WaitForApprovalManyResponse, error)
	ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (<-chan *ListManyResponse, error)
	GetOneMany(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (<-chan *GetManyResponse, error)
	ClearOneMany(ctx context.Context, in *ClearRequest, opts ...grpc.CallOption) (<-chan *ClearManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type mpaClientProxy struct {
	*mpaClient
}

// NewMpaClientProxy creates a MpaClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewMpaClientProxy(cc *proxy.Conn) MpaClientProxy {
	return &mpaClientProxy{NewMpaClient(cc).(*mpaClient)}
}

// StoreManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type StoreManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *StoreResponse
	Error error
}

// StoreOneMany provides the same API as Store but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *mpaClientProxy) StoreOneMany(ctx context.Context, in *StoreRequest, opts ...grpc.CallOption) (<-chan *StoreManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *StoreManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &StoreManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &StoreResponse{},
			}
			err := conn.Invoke(ctx, "/Mpa.Mpa/Store", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Mpa.Mpa/Store", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &StoreManyResponse{
				Resp: &StoreResponse{},
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

// ApproveManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type ApproveManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *ApproveResponse
	Error error
}

// ApproveOneMany provides the same API as Approve but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *mpaClientProxy) ApproveOneMany(ctx context.Context, in *ApproveRequest, opts ...grpc.CallOption) (<-chan *ApproveManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *ApproveManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ApproveManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ApproveResponse{},
			}
			err := conn.Invoke(ctx, "/Mpa.Mpa/Approve", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Mpa.Mpa/Approve", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ApproveManyResponse{
				Resp: &ApproveResponse{},
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

// WaitForApprovalManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type WaitForApprovalManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *WaitForApprovalResponse
	Error error
}

// WaitForApprovalOneMany provides the same API as WaitForApproval but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *mpaClientProxy) WaitForApprovalOneMany(ctx context.Context, in *WaitForApprovalRequest, opts ...grpc.CallOption) (<-chan *WaitForApprovalManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *WaitForApprovalManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &WaitForApprovalManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &WaitForApprovalResponse{},
			}
			err := conn.Invoke(ctx, "/Mpa.Mpa/WaitForApproval", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Mpa.Mpa/WaitForApproval", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &WaitForApprovalManyResponse{
				Resp: &WaitForApprovalResponse{},
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
	Resp  *ListResponse
	Error error
}

// ListOneMany provides the same API as List but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *mpaClientProxy) ListOneMany(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (<-chan *ListManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *ListManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ListManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ListResponse{},
			}
			err := conn.Invoke(ctx, "/Mpa.Mpa/List", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Mpa.Mpa/List", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ListManyResponse{
				Resp: &ListResponse{},
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

// GetManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type GetManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *GetResponse
	Error error
}

// GetOneMany provides the same API as Get but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *mpaClientProxy) GetOneMany(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (<-chan *GetManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *GetManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &GetManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &GetResponse{},
			}
			err := conn.Invoke(ctx, "/Mpa.Mpa/Get", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Mpa.Mpa/Get", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &GetManyResponse{
				Resp: &GetResponse{},
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

// ClearManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type ClearManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *ClearResponse
	Error error
}

// ClearOneMany provides the same API as Clear but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *mpaClientProxy) ClearOneMany(ctx context.Context, in *ClearRequest, opts ...grpc.CallOption) (<-chan *ClearManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *ClearManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ClearManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ClearResponse{},
			}
			err := conn.Invoke(ctx, "/Mpa.Mpa/Clear", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Mpa.Mpa/Clear", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ClearManyResponse{
				Resp: &ClearResponse{},
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
