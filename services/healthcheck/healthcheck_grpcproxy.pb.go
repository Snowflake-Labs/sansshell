// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package healthcheck

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"fmt"
)

// HealthCheckClientProxy is the superset of HealthCheckClient which additionally includes the OneMany proxy methods
type HealthCheckClientProxy interface {
	HealthCheckClient
	OkOneMany(ctx context.Context, in *Empty, opts ...grpc.CallOption) (<-chan *OkManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type healthCheckClientProxy struct {
	*healthCheckClient
}

// NewHealthCheckClientProxy creates a HealthCheckClientProxy for use in proxied connections.
// NOTE: This takes a ProxyConn instead of a generic ClientConnInterface as the methods here are only valid in ProxyConn contexts.
func NewHealthCheckClientProxy(cc *proxy.ProxyConn) HealthCheckClientProxy {
	return &healthCheckClientProxy{NewHealthCheckClient(cc).(*healthCheckClient)}
}

type OkManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *Empty
	Error error
}

// OkOneMany provides the same API as Ok but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *healthCheckClientProxy) OkOneMany(ctx context.Context, in *Empty, opts ...grpc.CallOption) (<-chan *OkManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *OkManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &OkManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &Empty{},
			}
			err := conn.Invoke(ctx, "/HealthCheck.HealthCheck/Ok", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/HealthCheck.HealthCheck/Ok", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &OkManyResponse{
				Resp: &Empty{},
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
