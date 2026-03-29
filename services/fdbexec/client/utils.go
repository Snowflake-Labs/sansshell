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

package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/fdbexec"
)

// FdbExecRemoteCommand is an options to exec a command on a remote host
type fdbExecRemoteOptions struct {
	// execAsUser, Optional. User is the user to run the command as, if nil the command will be run as the default user
	execAsUser *string
}

type FdbExecRemoteOption func(*fdbExecRemoteOptions)

func FdbExecAsUser(user string) FdbExecRemoteOption {
	return func(o *fdbExecRemoteOptions) {
		o.execAsUser = &user
	}
}

// FdbExecRemoteCommand is a helper function for execing a command on a remote host
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
//
// Deprecated: Use FdbExecRemoteCommandWithOpts instead.
func FdbExecRemoteCommand(ctx context.Context, conn *proxy.Conn, binary string, args ...string) (*pb.FdbExecResponse, error) {
	return FdbExecRemoteCommandWithOpts(ctx, conn, binary, args)
}

// FdbExecRemoteCommandWithOpts is a helper function for execing a command on a remote host with provided opts
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func FdbExecRemoteCommandWithOpts(ctx context.Context, conn *proxy.Conn, binary string, args []string, opts ...FdbExecRemoteOption) (*pb.FdbExecResponse, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("FdbExecRemoteCommand only supports single targets")
	}

	result, err := FdbExecRemoteCommandManyWithOpts(ctx, conn, binary, args)
	if err != nil {
		return nil, err
	}
	if len(result) < 1 {
		return nil, fmt.Errorf("FdbExecRemoteCommand error: received empty response")
	}
	if result[0].Error != nil {
		return nil, fmt.Errorf("FdbExecRemoteCommand error: %v", result[0].Error)
	}
	return result[0].Resp, nil
}

// FdbExecRemoteCommandMany is a helper function for execing a command on one or remote hosts
// using a proxy.Conn.
// `binary` refers to the absolute path of the binary file on the remote host.
//
// // Deprecated: Use FdbExecRemoteCommandManyWithOpts instead.
func FdbExecRemoteCommandMany(ctx context.Context, conn *proxy.Conn, binary string, args ...string) ([]*pb.RunManyResponse, error) {
	return FdbExecRemoteCommandManyWithOpts(ctx, conn, binary, args)
}

// FdbExecRemoteCommandManyWithOpts is a helper function for execing a command on one or remote hosts with provided opts
// using a proxy.Conn.
// `binary` refers to the absolute path of the binary file on the remote host.
func FdbExecRemoteCommandManyWithOpts(ctx context.Context, conn *proxy.Conn, binary string, args []string, opts ...FdbExecRemoteOption) ([]*pb.RunManyResponse, error) {
	execOptions := &fdbExecRemoteOptions{}
	for _, opt := range opts {
		opt(execOptions)
	}

	c := pb.NewFdbExecClientProxy(conn)
	req := &pb.FdbExecRequest{
		Command: binary,
		Args:    args,
	}

	if execOptions.execAsUser != nil {
		req.User = *execOptions.execAsUser
	}

	respChan, err := c.RunOneMany(ctx, req)
	if err != nil {
		return nil, err
	}
	result := make([]*pb.RunManyResponse, len(conn.Targets))
	for r := range respChan {
		result[r.Index] = r
	}

	return result, nil
}
