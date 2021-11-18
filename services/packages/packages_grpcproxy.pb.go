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
	UpdateOneMany(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (<-chan *UpdateManyResponse, error)
	ListInstalledOneMany(ctx context.Context, in *ListInstalledRequest, opts ...grpc.CallOption) (<-chan *ListInstalledManyResponse, error)
	RepoListOneMany(ctx context.Context, in *RepoListRequest, opts ...grpc.CallOption) (<-chan *RepoListManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type packagesClientProxy struct {
	*packagesClient
}

// NewPackagesClientProxy creates a PackagesClientProxy for use in proxied connections.
// NOTE: This takes a ProxyConn instead of a generic ClientConnInterface as the methods here are only valid in ProxyConn contexts.
func NewPackagesClientProxy(cc *proxy.ProxyConn) PackagesClientProxy {
	return &packagesClientProxy{NewPackagesClient(cc).(*packagesClient)}
}

type InstallManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *InstallReply
	Error error
}

// InstallOneMany provides the same API as Install but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) InstallOneMany(ctx context.Context, in *InstallRequest, opts ...grpc.CallOption) (<-chan *InstallManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *InstallManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
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

type UpdateManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *UpdateReply
	Error error
}

// UpdateOneMany provides the same API as Update but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) UpdateOneMany(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (<-chan *UpdateManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *UpdateManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
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

type ListInstalledManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *ListInstalledReply
	Error error
}

// ListInstalledOneMany provides the same API as ListInstalled but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) ListInstalledOneMany(ctx context.Context, in *ListInstalledRequest, opts ...grpc.CallOption) (<-chan *ListInstalledManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *ListInstalledManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
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

type RepoListManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *RepoListReply
	Error error
}

// RepoListOneMany provides the same API as RepoList but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *packagesClientProxy) RepoListOneMany(ctx context.Context, in *RepoListRequest, opts ...grpc.CallOption) (<-chan *RepoListManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *RepoListManyResponse)
	// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.
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
