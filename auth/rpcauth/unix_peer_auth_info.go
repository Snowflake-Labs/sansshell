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

package rpcauth

import (
	"google.golang.org/grpc/credentials"
)

// UnixPeerCreds represents the credentials of a Unix peer.
type UnixPeerCredentials struct {
	Uid        int
	Gids       []int // Primary and supplementary group IDs.
	UserName   string
	GroupNames []string
}

// UnixPeerAuthInfo contains the authentication information for a Unix peer,
// in a form suitable for authentication info returned by gRPC transport credentials.
type UnixPeerAuthInfo struct {
	credentials.CommonAuthInfo
	Credentials UnixPeerCredentials
}

func (UnixPeerAuthInfo) AuthType() string {
	return "insecure_with_unix_creds"
}
