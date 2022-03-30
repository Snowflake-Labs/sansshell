/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

// Needs to be in a test package as the protobuf imports proxy which would then
// cause a circular import.
package proxy_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	proxypb "github.com/Snowflake-Labs/sansshell/proxy"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/Snowflake-Labs/sansshell/proxy/server"
	tdpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
	"github.com/Snowflake-Labs/sansshell/proxy/testutil"
	tu "github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func startTestProxy(ctx context.Context, t *testing.T, targets map[string]*bufconn.Listener) map[string]*bufconn.Listener {
	t.Helper()
	targetDialer := server.NewDialer(testutil.WithBufDialer(targets), grpc.WithTransportCredentials(insecure.NewCredentials()))
	lis := bufconn.Listen(testutil.BufSize)
	authz := testutil.NewAllowAllRPCAuthorizer(ctx, t)
	grpcServer := grpc.NewServer(grpc.StreamInterceptor(authz.AuthorizeStream))
	proxyServer := server.New(targetDialer, authz)
	proxyServer.Register(grpcServer)
	go func() {
		// Don't care about errors here as they might come on shutdown and we
		// can't log through t at that point anyways.
		grpcServer.Serve(lis)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
	})
	return map[string]*bufconn.Listener{"proxy": lis}
}

func TestDial(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:123")
	startTestProxy(ctx, t, testServerMap)

	// This should fail since we don't set credentials
	_, err := proxy.DialContext(ctx, "b", []string{"foo:123"})
	tu.FatalOnNoErr("DialContext", err, t)

	for _, tc := range []struct {
		name    string
		proxy   string
		targets []string
		options []grpc.DialOption
		wantErr bool
	}{
		{
			name:    "proxy and N hosts",
			proxy:   "proxy",
			targets: []string{"foo:123", "bar:123"},
			options: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		},
		{
			name:    "no proxy and a host",
			targets: []string{"foo:123"},
			options: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		},
		{
			name:    "proxy and 1 host",
			proxy:   "proxy",
			targets: []string{"foo:123"},
			options: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		},
		{
			name:    "no proxy and N hosts",
			targets: []string{"foo:123", "bar:123"},
			wantErr: true,
			options: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		},
		{
			name:    "proxy and no targets",
			proxy:   "proxy",
			options: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		},
		{
			name:    "no proxy no targets",
			wantErr: true,
			options: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		},
		{
			name:    "no security set",
			proxy:   "proxy",
			targets: []string{"foo:123", "bar:123"},
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := proxy.Dial(tc.proxy, tc.targets, tc.options...)
			tu.WantErr(tc.name, err, tc.wantErr, t)
		})
	}
}

func TestUnary(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:123")
	bufMap := startTestProxy(ctx, t, testServerMap)

	// Combines the 2 maps so we can dial everything directly if needed.
	for k, v := range testServerMap {
		bufMap[k] = v
	}

	for _, tc := range []struct {
		name           string
		proxy          string
		targets        []string
		wantErrOneMany bool
		wantErr        bool
	}{
		{
			name:    "proxy N targets",
			proxy:   "proxy",
			targets: []string{"foo:123", "bar:123"},
			wantErr: true, // can't do unary call with N targets
		},
		{
			name:           "proxy N targets one down",
			proxy:          "proxy",
			targets:        []string{"foo:123", "bar:123", "baz:123"},
			wantErr:        true, // can't do unary call with N targets
			wantErrOneMany: true,
		},
		{
			name:    "proxy 1 target",
			proxy:   "proxy",
			targets: []string{"foo:123"},
		},
		{
			name:    "no proxy 1 target",
			targets: []string{"foo:123"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conn, err := proxy.Dial(tc.proxy, tc.targets, testutil.WithBufDialer(bufMap), grpc.WithTransportCredentials(insecure.NewCredentials()))
			tu.FatalOnErr("Dial", err, t)

			ts := tdpb.NewTestServiceClientProxy(conn)
			resp, err := ts.TestUnaryOneMany(context.Background(), &tdpb.TestRequest{Input: "input"})
			t.Log(err)

			for r := range resp {
				t.Logf("%+v", r)
				if !tc.wantErrOneMany {
					tu.FatalOnErr(fmt.Sprintf("target %s", r.Target), r.Error, t)
				}
			}

			_, err = ts.TestUnary(context.Background(), &tdpb.TestRequest{Input: "input"})
			t.Log(err)
			tu.WantErr(tc.name, err, tc.wantErr, t)

			resp, err = ts.TestUnaryOneMany(context.Background(), &tdpb.TestRequest{Input: "error"})
			tu.FatalOnErr("TestUnaryOneMany error", err, t)
			for r := range resp {
				t.Logf("%+v", r)
				tu.FatalOnNoErr(fmt.Sprintf("target %s", r.Target), r.Error, t)
			}

			// Check pass through cases
			if tc.proxy != "" {
				_, err = ts.TestUnary(context.Background(), &tdpb.TestRequest{Input: "error"})
				tu.FatalOnNoErr("TestUnary error", err, t)
			}

			// Do some direct calls against the conn to get at error cases
			resp2, err := conn.InvokeOneMany(context.Background(), "bad_method", &tdpb.TestRequest{Input: "input"})
			if tc.proxy == "" {
				tu.FatalOnNoErr("InvokeOneMany bad msg", err, t)
			} else {
				tu.FatalOnErr("InvokeOneMany bad msg", err, t)
				for r := range resp2 {
					tu.FatalOnNoErr("InvokeOneMany bad method", r.Error, t)
				}
			}
			_, err = conn.InvokeOneMany(context.Background(), "/Testdata.TestService/TestUnary", nil)
			tu.FatalOnNoErr("InvokeOneMany bad msg", err, t)

			err = conn.Close()
			tu.FatalOnErr("conn Close()", err, t)
		})
	}
}

func TestStreaming(t *testing.T) {
	ctx := context.Background()
	testServerMap := testutil.StartTestDataServers(t, "foo:123", "bar:123")
	bufMap := startTestProxy(ctx, t, testServerMap)

	// Combines the 2 maps so we can dial everything directly if needed.
	for k, v := range testServerMap {
		bufMap[k] = v
	}

	for _, tc := range []struct {
		name           string
		proxy          string
		targets        []string
		wantErrOneMany bool
		wantErr        bool
	}{
		{
			name:    "proxy N targets",
			proxy:   "proxy",
			targets: []string{"foo:123", "bar:123"},
			wantErr: true,
		},
		{
			name:    "proxy 1 target",
			proxy:   "proxy",
			targets: []string{"foo:123"},
		},
		{
			name:    "no proxy 1 target",
			targets: []string{"foo:123"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conn, err := proxy.Dial(tc.proxy, tc.targets, testutil.WithBufDialer(bufMap), grpc.WithTransportCredentials(insecure.NewCredentials()))
			tu.FatalOnErr("Dial", err, t)

			ts := tdpb.NewTestServiceClientProxy(conn)
			stream, err := ts.TestBidiStreamOneMany(context.Background())
			tu.FatalOnErr("getting stream", err, t)

			// Should always fail for proxy. For direct these may hang up so don't bother.
			if tc.proxy != "" {
				_, err = stream.Header()
				tu.FatalOnNoErr("Header on proxy stream", err, t)

				// Should always return nil for proxy
				if td := stream.Trailer(); td != nil {
					t.Fatalf("unexpected response from Trailer(): %v", td)
				}
			}

			// In the proxy case do some bad messages which won't send anything
			if tc.proxy != "" {
				err = stream.SendMsg(nil)
				tu.FatalOnNoErr("SendMsg with nil", err, t)

				err = stream.RecvMsg(nil)
				tu.FatalOnNoErr("RecvMsg with nil", err, t)
			}

			// Should return a context
			if c := stream.Context(); c == nil {
				t.Fatal("Context() returned nil")
			}

			// Send one normal and one error. We should get back N replies, # targets errors in that.
			err = stream.Send(&tdpb.TestRequest{Input: "input"})
			tu.FatalOnErr("Send", err, t)
			err = stream.Send(&tdpb.TestRequest{Input: "error"})
			tu.FatalOnErr("Send error", err, t)

			// Shouldn't fail even twice.
			err = stream.CloseSend()
			tu.FatalOnErr("CloseSend", err, t)
			err = stream.CloseSend()
			tu.FatalOnErr("CloseSend", err, t)

			errors := 0

			for {
				resp, err := stream.Recv()
				if err != nil && err == io.EOF {
					break
				}
				tu.FatalOnErr("Recv", err, t)
				for _, r := range resp {
					t.Logf("%+v", r)
					if r.Error == io.EOF {
						continue
					}
					if r.Error != nil {
						errors++
					}
				}
			}

			if got, want := errors, len(tc.targets); got != want {
				t.Fatalf("Invalid error count, got %d want %d", got, want)
			}

			// Should give an error now
			err = stream.Send(&tdpb.TestRequest{Input: "input"})
			tu.FatalOnNoErr("Send on closed", err, t)
			conn.Close()
		})
	}

}

type fakeProxy struct {
	action func(proxypb.Proxy_ProxyServer) error
}

func (f *fakeProxy) Proxy(stream proxypb.Proxy_ProxyServer) error {
	return f.action(stream)
}

func TestWithFakeServerForErrors(t *testing.T) {
	// Setup our fake server.
	ctx := context.Background()
	lis := bufconn.Listen(testutil.BufSize)
	authz := testutil.NewAllowAllRPCAuthorizer(ctx, t)
	grpcServer := grpc.NewServer(grpc.StreamInterceptor(authz.AuthorizeStream))
	fp := &fakeProxy{}
	proxypb.RegisterProxyServer(grpcServer, fp)
	go func() {
		// Don't care about errors here as they might come on shutdown and we
		// can't log through t at that point anyways.
		grpcServer.Serve(lis)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
	})

	bd := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	errorFunc := func(proxypb.Proxy_ProxyServer) error { return errors.New("error") }
	channelSetup := func(stream proxypb.Proxy_ProxyServer) error {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		stream.Send(&proxypb.ProxyReply{
			Reply: &proxypb.ProxyReply_StartStreamReply{
				StartStreamReply: &proxypb.StartStreamReply{
					Target: req.GetStartStream().Target,
					Nonce:  req.GetStartStream().Nonce,
					Reply: &proxypb.StartStreamReply_StreamId{
						StreamId: 0,
					},
				},
			},
		})
		return nil
	}
	setupThenError := func(stream proxypb.Proxy_ProxyServer) error {
		channelSetup(stream)
		_, err := stream.Recv()
		if err != nil {
			return err
		}
		return errors.New("error")
	}
	setupThenEOF := func(stream proxypb.Proxy_ProxyServer) error {
		channelSetup(stream)
		_, err := stream.Recv()
		if err != nil {
			return err
		}
		return nil
	}

	notStartReply := func(stream proxypb.Proxy_ProxyServer) error {
		_, err := stream.Recv()
		if err != nil {
			return err
		}
		stream.Send(&proxypb.ProxyReply{
			Reply: &proxypb.ProxyReply_ServerClose{},
		})
		return nil
	}
	nonMatchingData := func(stream proxypb.Proxy_ProxyServer) error {
		_, err := stream.Recv()
		if err != nil {
			return err
		}
		stream.Send(&proxypb.ProxyReply{
			Reply: &proxypb.ProxyReply_StartStreamReply{
				StartStreamReply: &proxypb.StartStreamReply{},
			},
		})
		return nil
	}
	dataPacketWrongID := func(stream proxypb.Proxy_ProxyServer) error {
		channelSetup(stream)
		_, err := stream.Recv()
		if err != nil {
			return err
		}
		stream.Send(&proxypb.ProxyReply{
			Reply: &proxypb.ProxyReply_StreamData{
				StreamData: &proxypb.StreamData{
					StreamIds: []uint64{1},
					Payload:   &anypb.Any{},
				},
			},
		})
		return nil
	}
	closePacketWrongID := func(stream proxypb.Proxy_ProxyServer) error {
		channelSetup(stream)
		_, err := stream.Recv()
		if err != nil {
			return err
		}
		stream.Send(&proxypb.ProxyReply{
			Reply: &proxypb.ProxyReply_ServerClose{
				ServerClose: &proxypb.ServerClose{
					StreamIds: []uint64{1},
				},
			},
		})
		return nil
	}
	badPacket := func(stream proxypb.Proxy_ProxyServer) error {
		channelSetup(stream)
		_, err := stream.Recv()
		if err != nil {
			return err
		}
		stream.Send(&proxypb.ProxyReply{})
		return nil
	}
	validReplyThenCloseError := func(stream proxypb.Proxy_ProxyServer) error {
		channelSetup(stream)
		_, err := stream.Recv()
		if err != nil {
			return err
		}
		payload, err := anypb.New(&emptypb.Empty{})
		if err != nil {
			return err
		}
		stream.Send(&proxypb.ProxyReply{
			Reply: &proxypb.ProxyReply_StreamData{
				StreamData: &proxypb.StreamData{
					Payload:   payload,
					StreamIds: []uint64{0},
				},
			},
		})
		stream.Send(&proxypb.ProxyReply{
			Reply: &proxypb.ProxyReply_ServerClose{
				ServerClose: &proxypb.ServerClose{
					Status: &proxypb.Status{
						Code: int32(codes.Aborted),
					},
					StreamIds: []uint64{0},
				},
			},
		})
		_, err = stream.Recv()
		if err != nil {
			return err
		}
		_, err = stream.Recv()
		if err != nil {
			return err
		}

		return nil
	}

	for _, tc := range []struct {
		name          string
		action        func(stream proxypb.Proxy_ProxyServer) error
		input         proto.Message
		output        proto.Message
		wantErr       bool
		wantStreamErr bool
	}{
		{
			name:          "base error on send",
			action:        errorFunc,
			input:         &emptypb.Empty{},
			output:        &emptypb.Empty{},
			wantErr:       true,
			wantStreamErr: true,
		},
		{
			name:    "early EOF",
			action:  channelSetup,
			input:   &emptypb.Empty{},
			output:  &emptypb.Empty{},
			wantErr: true,
		},
		{
			name:          "invalid reply",
			action:        notStartReply,
			input:         &emptypb.Empty{},
			output:        &emptypb.Empty{},
			wantErr:       true,
			wantStreamErr: true,
		},
		{
			name:          "don't match target/nonce",
			action:        nonMatchingData,
			input:         &emptypb.Empty{},
			output:        &emptypb.Empty{},
			wantErr:       true,
			wantStreamErr: true,
		},
		{
			name:    "valid then error",
			action:  setupThenError,
			input:   &emptypb.Empty{},
			output:  &emptypb.Empty{},
			wantErr: true,
		},
		{
			name:    "setup then EOF but nil response",
			action:  setupThenEOF,
			input:   &emptypb.Empty{},
			output:  &emptypb.Empty{},
			wantErr: true,
		},
		{
			name:    "setup then EOF but bad output",
			action:  setupThenEOF,
			input:   &emptypb.Empty{},
			wantErr: true,
		},
		{
			name:    "data packet wrong id",
			action:  dataPacketWrongID,
			input:   &emptypb.Empty{},
			output:  &emptypb.Empty{},
			wantErr: true,
		},
		{
			name:    "close packet wrong id",
			action:  closePacketWrongID,
			input:   &emptypb.Empty{},
			output:  &emptypb.Empty{},
			wantErr: true,
		},
		{
			name:    "bad packet - not close or data",
			action:  badPacket,
			input:   &emptypb.Empty{},
			output:  &emptypb.Empty{},
			wantErr: true,
		},
		{
			name:   "valid reply then close error",
			action: validReplyThenCloseError,
			input:  &emptypb.Empty{},
			output: &emptypb.Empty{},
			// No error for this as it should eat it internally.
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conn, err := proxy.DialContext(ctx, "bufnet", []string{"foo:123"}, grpc.WithContextDialer(bd), grpc.WithTransportCredentials(insecure.NewCredentials()))
			tu.FatalOnErr("DialContext", err, t)

			fp.action = tc.action
			err = conn.Invoke(ctx, "/method", tc.input, tc.output)
			t.Log(err)
			tu.WantErr(tc.name, err, tc.wantErr, t)

			_, err = conn.NewStream(ctx, &grpc.StreamDesc{}, "/method")
			tu.WantErr(tc.name+" NewStream", err, tc.wantStreamErr, t)
		})
	}
}
