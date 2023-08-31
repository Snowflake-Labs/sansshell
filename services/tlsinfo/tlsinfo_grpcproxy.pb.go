// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package tlsinfo

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"fmt"
)

// TLSInfoClientProxy is the superset of TLSInfoClient which additionally includes the OneMany proxy methods
type TLSInfoClientProxy interface {
	TLSInfoClient
	GetTLSCertificateOneMany(ctx context.Context, in *TLSCertificateRequest, opts ...grpc.CallOption) (<-chan *GetTLSCertificateManyResponse, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type tLSInfoClientProxy struct {
	*tLSInfoClient
}

// NewTLSInfoClientProxy creates a TLSInfoClientProxy for use in proxied connections.
// NOTE: This takes a proxy.Conn instead of a generic ClientConnInterface as the methods here are only valid in proxy.Conn contexts.
func NewTLSInfoClientProxy(cc *proxy.Conn) TLSInfoClientProxy {
	return &tLSInfoClientProxy{NewTLSInfoClient(cc).(*tLSInfoClient)}
}

// GetTLSCertificateManyResponse encapsulates a proxy data packet.
// It includes the target, index, response and possible error returned.
type GetTLSCertificateManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to proxy.Conn.
	Index int
	Resp  *TLSCertificateChain
	Error error
}

// GetTLSCertificateOneMany provides the same API as GetTLSCertificate but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *tLSInfoClientProxy) GetTLSCertificateOneMany(ctx context.Context, in *TLSCertificateRequest, opts ...grpc.CallOption) (<-chan *GetTLSCertificateManyResponse, error) {
	conn := c.cc.(*proxy.Conn)
	ret := make(chan *GetTLSCertificateManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &GetTLSCertificateManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &TLSCertificateChain{},
			}
			err := conn.Invoke(ctx, "/TLSInfo.TLSInfo/GetTLSCertificate", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/TLSInfo.TLSInfo/GetTLSCertificate", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &GetTLSCertificateManyResponse{
				Resp: &TLSCertificateChain{},
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
