//go:build linux

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
	"fmt"
	"net"
	"os/user"
	"strconv"

	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"golang.org/x/sys/unix"
)

// getUnixPeerCredentials returns the peer's Unix credentials from the given network connection.
//
// The provided connection should be established over a Unix domain socket.
func getUnixPeerCredentials(conn net.Conn) (*rpcauth.UnixPeerCredentials, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("called getUnixPeerCredentials on non-Unix connection")
	}

	rawConn, err := uc.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw connection: %w", err)
	}

	var ucred *unix.Ucred
	err2 := rawConn.Control(func(fd uintptr) {
		ucred, err = unix.GetsockoptUcred(int(fd),
			unix.SOL_SOCKET,
			unix.SO_PEERCRED)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get peer credentials - getsockopt error: %w", err)
	}
	if err2 != nil {
		return nil, fmt.Errorf("failed to get peer credentials - socket Control error: %w", err2)
	}

	// Convert UID to user name, fetch primary & supplementary group IDs and convert them to group
	// names. If any user/group lookup fails, use the numeric value.
	uid := int(ucred.Uid)
	userName := strconv.Itoa(uid)
	userInfo, err := user.LookupId(userName)
	if err == nil {
		userName = userInfo.Username
	}

	groupIdStrings, err := userInfo.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("failed to get group IDs for user %s: %w", userName, err)
	}
	groupIds := []int{}
	groupNames := []string{}

	for _, groupIdString := range groupIdStrings {
		groupId, err := strconv.Atoi(groupIdString)
		if err != nil {
			return nil, fmt.Errorf("failed to convert group ID %s to int: %w", groupIdString, err)
		}
		groupIds = append(groupIds, groupId)

		groupInfo, err := user.LookupGroupId(groupIdString)
		if err == nil {
			groupNames = append(groupNames, groupInfo.Name)
		} else {
			groupNames = append(groupNames, groupIdString)
		}
	}

	return &rpcauth.UnixPeerCredentials{
		Uid:        uid,
		Gids:       groupIds,
		UserName:   userName,
		GroupNames: groupNames,
	}, nil
}
