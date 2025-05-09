/* Copyright (c) 2025 Snowflake Inc. All rights reserved.

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

package rpcauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"net"
	"net/url"
	"testing"
)

type testAuthInfo struct {
	credentials.CommonAuthInfo
}

func (testAuthInfo) AuthType() string {
	return "testAuthInfo"
}

func TestRpcAuthInput(t *testing.T) {
	md := metadata.New(map[string]string{
		"foo": "foo",
		"bar": "bar",
	})
	tcp, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:1")
	testutil.FatalOnErr("ResolveIPAddr", err, t)

	ctx := context.Background()

	for _, tc := range []struct {
		name    string
		ctx     context.Context
		method  string
		req     proto.Message
		compare *RPCAuthInput
	}{
		{
			name:   "method only",
			ctx:    ctx,
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer:   nil,
			},
		},
		{
			name:   "method and metadata",
			ctx:    metadata.NewIncomingContext(ctx, md),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method:   "/AMethod",
				Metadata: md,
				Peer:     nil,
			},
		},
		{
			name:   "method and request",
			ctx:    ctx,
			method: "/AMethod",
			req:    &emptypb.Empty{},
			compare: &RPCAuthInput{
				Method:      "/AMethod",
				Message:     json.RawMessage{0x7b, 0x7d},
				MessageType: "google.protobuf.Empty",
				Peer:        nil,
			},
		},
		{
			name:   "method and a peer context but no addr",
			ctx:    peer.NewContext(ctx, &peer.Peer{}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer:   &PeerAuthInput{},
			},
		},
		{
			name: "method and a peer context",
			ctx: peer.NewContext(ctx, &peer.Peer{
				Addr: tcp,
			}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer: &PeerAuthInput{
					Net: &NetAuthInput{
						Network: "tcp",
						Address: "127.0.0.1",
						Port:    "1",
					},
				},
			},
		},
		{
			name: "method and a peer context with non tls auth",
			ctx: peer.NewContext(ctx, &peer.Peer{
				Addr:     tcp,
				AuthInfo: testAuthInfo{},
			}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer: &PeerAuthInput{
					Net: &NetAuthInput{
						Network: "tcp",
						Address: "127.0.0.1",
						Port:    "1",
					},
					Cert: &CertAuthInput{},
				},
			},
		},
		{
			name: "method and a peer context with tls auth",
			ctx: peer.NewContext(ctx, &peer.Peer{
				Addr: tcp,
				AuthInfo: credentials.TLSInfo{
					SPIFFEID: &url.URL{
						Path: "/",
					},
					State: tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{
							{}, // Don't need an actual cert, a placeholder will work.
						},
					},
				},
			}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer: &PeerAuthInput{
					Net: &NetAuthInput{
						Network: "tcp",
						Address: "127.0.0.1",
						Port:    "1",
					},
					Cert: &CertAuthInput{SPIFFEID: "/"},
				},
			},
		},
		{
			name: "method and a peer context with unix creds",
			ctx: peer.NewContext(ctx, &peer.Peer{
				Addr: &net.UnixAddr{Net: "unix", Name: "@"},
				AuthInfo: UnixPeerAuthInfo{
					CommonAuthInfo: credentials.CommonAuthInfo{
						SecurityLevel: credentials.NoSecurity,
					},
					Credentials: UnixPeerCredentials{
						Uid:        1,
						UserName:   "george",
						Gids:       []int{1001, 2},
						GroupNames: []string{"george", "the_gang"},
					},
				},
			}),
			method: "/AMethod",
			compare: &RPCAuthInput{
				Method: "/AMethod",
				Peer: &PeerAuthInput{
					Net: &NetAuthInput{
						Network: "unix",
						Address: "@",
						Port:    "",
					},
					Unix: &UnixAuthInput{
						Uid:        1,
						UserName:   "george",
						Gids:       []int{1001, 2},
						GroupNames: []string{"george", "the_gang"},
					},
					Cert: &CertAuthInput{},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rpcauth, err := NewRPCAuthInput(tc.ctx, tc.method, tc.req)
			testutil.FatalOnErr(tc.name, err, t)
			testutil.DiffErr(tc.name, rpcauth, tc.compare, t)
		})
	}
}
