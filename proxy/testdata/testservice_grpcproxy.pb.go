// Auto generated code by protoc-gen-go-grpcproxy
// DO NOT EDIT

// Adds OneMany versions of RPC methods for use by proxy clients

package testdata

import (
	context "context"
	proxy "github.com/Snowflake-Labs/sansshell/proxy/proxy"
	grpc "google.golang.org/grpc"
)

import (
	"fmt"
)

// TestServiceClientProxy is the superset of TestServiceClient which additionally includes the OneMany proxy methods
type TestServiceClientProxy interface {
	TestServiceClient
	TestUnaryOneMany(ctx context.Context, in *TestRequest, opts ...grpc.CallOption) (<-chan *TestUnaryManyResponse, error)
	TestServerStreamOneMany(ctx context.Context, in *TestRequest, opts ...grpc.CallOption) (TestService_TestServerStreamClientProxy, error)
	TestClientStreamOneMany(ctx context.Context, opts ...grpc.CallOption) (TestService_TestClientStreamClientProxy, error)
	TestBidiStreamOneMany(ctx context.Context, opts ...grpc.CallOption) (TestService_TestBidiStreamClientProxy, error)
}

// Embed the original client inside of this so we get the other generated methods automatically.
type testServiceClientProxy struct {
	*testServiceClient
}

// NewTestServiceClientProxy creates a TestServiceClientProxy for use in proxied connections.
// NOTE: This takes a ProxyConn instead of a generic ClientConnInterface as the methods here are only valid in ProxyConn contexts.
func NewTestServiceClientProxy(cc *proxy.ProxyConn) TestServiceClientProxy {
	return &testServiceClientProxy{NewTestServiceClient(cc).(*testServiceClient)}
}

type TestUnaryManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *TestResponse
	Error error
}

// TestUnaryOneMany provides the same API as TestUnary but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *testServiceClientProxy) TestUnaryOneMany(ctx context.Context, in *TestRequest, opts ...grpc.CallOption) (<-chan *TestUnaryManyResponse, error) {
	conn := c.cc.(*proxy.ProxyConn)
	ret := make(chan *TestUnaryManyResponse)
	// If this is a single case we can just use Invoke and marshal it onto the channel once and be done.
	if len(conn.Targets) == 1 {
		go func() {
			out := &TestUnaryManyResponse{
				Target: conn.Targets[0],
				Index:  0,
				Resp:   &TestResponse{},
			}
			err := conn.Invoke(ctx, "/Testdata.TestService/TestUnary", in, out.Resp, opts...)
			if err != nil {
				out.Error = err
			}
			// Send and close.
			ret <- out
			close(ret)
		}()
		return ret, nil
	}
	manyRet, err := conn.InvokeOneMany(ctx, "/Testdata.TestService/TestUnary", in, opts...)
	if err != nil {
		return nil, err
	}
	// A goroutine to retrive untyped responses and convert them to typed ones.
	go func() {
		for {
			typedResp := &TestUnaryManyResponse{
				Resp: &TestResponse{},
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

type TestServerStreamManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *TestResponse
	Error error
}

type TestService_TestServerStreamClientProxy interface {
	Recv() ([]*TestServerStreamManyResponse, error)
	grpc.ClientStream
}

type testServiceClientTestServerStreamClientProxy struct {
	cc *proxy.ProxyConn
	grpc.ClientStream
}

func (x *testServiceClientTestServerStreamClientProxy) Recv() ([]*TestServerStreamManyResponse, error) {
	var ret []*TestServerStreamManyResponse
	// If this is a direct connection the RecvMsg call is to a standard grpc.ClientStream
	// and not our proxy based one. This means we need to receive a typed response and
	// convert it into a single slice entry return. This ensures the OneMany style calls
	// can be used by proxy with 1:N targets and non proxy with 1 target without client changes.
	if x.cc.Direct() {
		m := &TestResponse{}
		if err := x.ClientStream.RecvMsg(m); err != nil {
			return nil, err
		}
		ret = append(ret, &TestServerStreamManyResponse{
			Resp:   m,
			Target: x.cc.Targets[0],
			Index:  0,
		})
		return ret, nil
	}

	m := []*proxy.ProxyRet{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	for _, r := range m {
		typedResp := &TestServerStreamManyResponse{
			Resp: &TestResponse{},
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

// TestServerStreamOneMany provides the same API as TestServerStream but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *testServiceClientProxy) TestServerStreamOneMany(ctx context.Context, in *TestRequest, opts ...grpc.CallOption) (TestService_TestServerStreamClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[0], "/Testdata.TestService/TestServerStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceClientTestServerStreamClientProxy{c.cc.(*proxy.ProxyConn), stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type TestClientStreamManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *TestResponse
	Error error
}

type TestService_TestClientStreamClientProxy interface {
	Send(*TestRequest) error
	CloseAndRecv() ([]*TestClientStreamManyResponse, error)
	grpc.ClientStream
}

type testServiceClientTestClientStreamClientProxy struct {
	cc *proxy.ProxyConn
	grpc.ClientStream
}

func (x *testServiceClientTestClientStreamClientProxy) Send(m *TestRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceClientTestClientStreamClientProxy) CloseAndRecv() ([]*TestClientStreamManyResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	var ret []*TestClientStreamManyResponse
	// If this is a direct connection the RecvMsg call is to a standard grpc.ClientStream
	// and not our proxy based one. This means we need to receive a typed response and
	// convert it into a single slice entry return. This ensures the OneMany style calls
	// can be used by proxy with 1:N targets and non proxy with 1 target without client changes.
	if x.cc.Direct() {
		m := &TestResponse{}
		if err := x.ClientStream.RecvMsg(m); err != nil {
			return nil, err
		}
		ret = append(ret, &TestClientStreamManyResponse{
			Resp:   m,
			Target: x.cc.Targets[0],
			Index:  0,
		})
		return ret, nil
	}

	m := []*proxy.ProxyRet{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	for _, r := range m {
		typedResp := &TestClientStreamManyResponse{
			Resp: &TestResponse{},
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

// TestClientStreamOneMany provides the same API as TestClientStream but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *testServiceClientProxy) TestClientStreamOneMany(ctx context.Context, opts ...grpc.CallOption) (TestService_TestClientStreamClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[1], "/Testdata.TestService/TestClientStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceClientTestClientStreamClientProxy{c.cc.(*proxy.ProxyConn), stream}
	return x, nil
}

type TestBidiStreamManyResponse struct {
	Target string
	// As targets can be duplicated this is the index into the slice passed to ProxyConn.
	Index int
	Resp  *TestResponse
	Error error
}

type TestService_TestBidiStreamClientProxy interface {
	Send(*TestRequest) error
	Recv() ([]*TestBidiStreamManyResponse, error)
	grpc.ClientStream
}

type testServiceClientTestBidiStreamClientProxy struct {
	cc *proxy.ProxyConn
	grpc.ClientStream
}

func (x *testServiceClientTestBidiStreamClientProxy) Send(m *TestRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceClientTestBidiStreamClientProxy) Recv() ([]*TestBidiStreamManyResponse, error) {
	var ret []*TestBidiStreamManyResponse
	// If this is a direct connection the RecvMsg call is to a standard grpc.ClientStream
	// and not our proxy based one. This means we need to receive a typed response and
	// convert it into a single slice entry return. This ensures the OneMany style calls
	// can be used by proxy with 1:N targets and non proxy with 1 target without client changes.
	if x.cc.Direct() {
		m := &TestResponse{}
		if err := x.ClientStream.RecvMsg(m); err != nil {
			return nil, err
		}
		ret = append(ret, &TestBidiStreamManyResponse{
			Resp:   m,
			Target: x.cc.Targets[0],
			Index:  0,
		})
		return ret, nil
	}

	m := []*proxy.ProxyRet{}
	if err := x.ClientStream.RecvMsg(&m); err != nil {
		return nil, err
	}
	for _, r := range m {
		typedResp := &TestBidiStreamManyResponse{
			Resp: &TestResponse{},
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

// TestBidiStreamOneMany provides the same API as TestBidiStream but sends the same request to N destinations at once.
// N can be a single destination.
//
// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.
func (c *testServiceClientProxy) TestBidiStreamOneMany(ctx context.Context, opts ...grpc.CallOption) (TestService_TestBidiStreamClientProxy, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[2], "/Testdata.TestService/TestBidiStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceClientTestBidiStreamClientProxy{c.cc.(*proxy.ProxyConn), stream}
	return x, nil
}
