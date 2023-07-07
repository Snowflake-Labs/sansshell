// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package packages

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"fmt"
)

// PackagesClientProxy is the superset of PackagesClient which additionally includes the OneMany proxy methods
type PackagesClientProxy interface {
	PackagesClient
	InstallOneMany(ctx context.Context, in *InstallRequest, opts ...grpc.CallOption) (<-chan *InstallManyResponse, error)
	RemoveOneMany(ctx context.Context, in *RemoveRequest, opts ...grpc.CallOption) (<-chan *RemoveManyResponse, error)
	UpdateOneMany(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (<-chan *UpdateManyResponse, error)
	SearchOneMany(ctx context.Context, in *SearchRequest, opts ...grpc.CallOption) (<-chan *SearchManyResponse, error)
	ListInstalledOneMany(ctx context.Context, in *ListInstalledRequest, opts ...grpc.CallOption) (<-chan *ListInstalledManyResponse, error)
	RepoListOneMany(ctx context.Context, in *RepoListRequest, opts ...grpc.CallOption) (<-chan *RepoListManyResponse, error)
	CleanupOneMany(ctx context.Context, in *CleanupRequest, opts ...grpc.CallOption) (<-chan *CleanupManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type packagesClientProxy struct {
	*packagesClient
}

// NewPackagesClientProxy creates a PackagesClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewPackagesClientProxy(cc *proxy.Conn) PackagesClientProxy {
	return &packagesClientProxy{NewPackagesClient(cc).(*packagesClient)}
}

// InstallManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type InstallManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *InstallReply
	Error error
}

// InstallOneMany provides the same API as Install but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) InstallOneMany(ctx context.Context, in *InstallRequest, opts ...grpc.CallOption) (<-chan *InstallManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *InstallManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &InstallManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &InstallReply{},
			}
			err := conn.Invoke(ctx, "/Packages.Packages/Install", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Packages.Packages/Install", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &InstallManyResponse{
				Resp: &InstallReply{},
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

// RemoveManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type RemoveManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *RemoveReply
	Error error
}

// RemoveOneMany provides the same API as Remove but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) RemoveOneMany(ctx context.Context, in *RemoveRequest, opts ...grpc.CallOption) (<-chan *RemoveManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *RemoveManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &RemoveManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &RemoveReply{},
			}
			err := conn.Invoke(ctx, "/Packages.Packages/Remove", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Packages.Packages/Remove", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &RemoveManyResponse{
				Resp: &RemoveReply{},
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

// UpdateManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type UpdateManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *UpdateReply
	Error error
}

// UpdateOneMany provides the same API as Update but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) UpdateOneMany(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (<-chan *UpdateManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *UpdateManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &UpdateManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &UpdateReply{},
			}
			err := conn.Invoke(ctx, "/Packages.Packages/Update", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Packages.Packages/Update", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &UpdateManyResponse{
				Resp: &UpdateReply{},
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

// SearchManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type SearchManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *SearchReply
	Error error
}

// SearchOneMany provides the same API as Search but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) SearchOneMany(ctx context.Context, in *SearchRequest, opts ...grpc.CallOption) (<-chan *SearchManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *SearchManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &SearchManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &SearchReply{},
			}
			err := conn.Invoke(ctx, "/Packages.Packages/Search", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Packages.Packages/Search", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &SearchManyResponse{
				Resp: &SearchReply{},
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

// ListInstalledManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type ListInstalledManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *ListInstalledReply
	Error error
}

// ListInstalledOneMany provides the same API as ListInstalled but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) ListInstalledOneMany(ctx context.Context, in *ListInstalledRequest, opts ...grpc.CallOption) (<-chan *ListInstalledManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *ListInstalledManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &ListInstalledManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &ListInstalledReply{},
			}
			err := conn.Invoke(ctx, "/Packages.Packages/ListInstalled", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Packages.Packages/ListInstalled", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &ListInstalledManyResponse{
				Resp: &ListInstalledReply{},
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

// RepoListManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type RepoListManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *RepoListReply
	Error error
}

// RepoListOneMany provides the same API as RepoList but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) RepoListOneMany(ctx context.Context, in *RepoListRequest, opts ...grpc.CallOption) (<-chan *RepoListManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *RepoListManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &RepoListManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &RepoListReply{},
			}
			err := conn.Invoke(ctx, "/Packages.Packages/RepoList", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Packages.Packages/RepoList", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &RepoListManyResponse{
				Resp: &RepoListReply{},
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

// CleanupManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type CleanupManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *CleanupResponse
	Error error
}

// CleanupOneMany provides the same API as Cleanup but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) CleanupOneMany(ctx context.Context, in *CleanupRequest, opts ...grpc.CallOption) (<-chan *CleanupManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *CleanupManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &CleanupManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &CleanupResponse{},
			}
			err := conn.Invoke(ctx, "/Packages.Packages/Cleanup", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Packages.Packages/Cleanup", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &CleanupManyResponse{
				Resp: &CleanupResponse{},
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
