//go:build darwin

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
	"github.com/Snowflake-Labs/sansshell/auth/rpcauthz"
	"net"
	"os/user"
	"strconv"

	"golang.org/x/sys/unix"
)

// getUnixPeerCredentials indicates missing Unix credentials on non-Linux systems.
//
// This is needed so that the rpcauth package compiles on non-Linux systems,
// where Unix credentials cannot be fetched.
func getUnixPeerCredentials(conn net.Conn) (*rpcauthz.UnixPeerCredentials, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("called getUnixPeerCredentials on non-Unix connection")
	}

	rawConn, err := uc.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw connection: %w", err)
	}

	var ucred *unix.Xucred
	err2 := rawConn.Control(func(fd uintptr) {
		ucred, err = unix.GetsockoptXucred(int(fd),
			unix.SOL_LOCAL,
			unix.LOCAL_PEERCRED)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get peer credentials - getsockopt error: %w", err)
	}
	if err2 != nil {
		return nil, fmt.Errorf("failed to get peer credentials - socket Control error: %w", err2)
	}

	// Convert UID and GIDs to user & group names. If any lookup fails, use the numeric value.
	uid := int(ucred.Uid)
	userName := strconv.Itoa(uid)
	userInfo, err := user.LookupId(userName)
	if err == nil {
		userName = userInfo.Username
	}

	groupIds := []int{}
	for _, groupId := range ucred.Groups[0:ucred.Ngroups] {
		groupIds = append(groupIds, int(groupId))
	}
	groupNames := []string{}

	for _, groupId := range groupIds {
		groupIdString := strconv.Itoa(groupId)
		groupInfo, err := user.LookupGroupId(groupIdString)
		if err == nil {
			groupNames = append(groupNames, groupInfo.Name)
		} else {
			groupNames = append(groupNames, groupIdString)
		}
	}

	return &rpcauthz.UnixPeerCredentials{
		Uid:        uid,
		Gids:       groupIds,
		UserName:   userName,
		GroupNames: groupNames,
	}, nil
}
