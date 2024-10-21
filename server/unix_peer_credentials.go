/* Copyright (c) 2024 Snowflake Inc. All rights reserved.

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

package server

import (
	"context"
	"fmt"
	"net"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// unixPeerTransportCredentials is a TransportCredentials implementation that fetches the
// peer's credentials from the Unix domain socket. Otherwise, the channel is insecure (no TLS).
type unixPeerTransportCredentials struct {
	insecureCredentials credentials.TransportCredentials
}

func (uc *unixPeerTransportCredentials) ClientHandshake(ctx context.Context, authority string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return uc.insecureCredentials.ClientHandshake(ctx, authority, conn)
}

func (uc *unixPeerTransportCredentials) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, insecureAuthInfo, err := uc.insecureCredentials.ServerHandshake(conn)
	if err != nil {
		return nil, nil, err
	}

	unixCreds, err := getUnixPeerCredentials(conn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get unix peer credentials: %w", err)
	}
	if unixCreds == nil {
		// This means Unix credentials are not available (not a Unix system).
		// We treat this connection as a basic insecure connection, with the
		// authentication info coming from the 'insecure' module.
		return conn, insecureAuthInfo, nil
	}

	unixPeerAuthInfo := rpcauth.UnixPeerAuthInfo{
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.NoSecurity},
		Credentials:    *unixCreds,
	}
	return conn, unixPeerAuthInfo, nil
}

func (uc *unixPeerTransportCredentials) Info() credentials.ProtocolInfo {
	return uc.insecureCredentials.Info()
}

func (uc *unixPeerTransportCredentials) Clone() credentials.TransportCredentials {
	return &unixPeerTransportCredentials{
		insecureCredentials: uc.insecureCredentials.Clone(),
	}
}

func (uc *unixPeerTransportCredentials) OverrideServerName(serverName string) error {
	// This is the same as the insecure implementation, but does not use
	// its deprecated method.
	return nil
}

// NewUnixPeerCredentials returns a new TransportCredentials that disables transport security,
// but fetches the peer's credentials from the Unix domain socket.
func NewUnixPeerTransportCredentials() credentials.TransportCredentials {
	return &unixPeerTransportCredentials{
		insecureCredentials: insecure.NewCredentials(),
	}
}
