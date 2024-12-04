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
	pb "github.com/Snowflake-Labs/sansshell/services/exec"
)

// ExecRemoteCommand is an options to exec a command on a remote host
type execRemoteOptions struct {
	// execAsUser, Optional. User is the user to run the command as, if nil the command will be run as the default user
	execAsUser *string
}

type ExecRemoteOption func(*execRemoteOptions)

func ExecAsUser(user string) ExecRemoteOption {
	return func(o *execRemoteOptions) {
		o.execAsUser = &user
	}
}

// ExecRemoteCommand is a helper function for execing a command on a remote host
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
//
// Deprecated: Use ExecRemoteCommandWithOpts instead.
func ExecRemoteCommand(ctx context.Context, conn *proxy.Conn, binary string, args ...string) (*pb.ExecResponse, error) {
	return ExecRemoteCommandWithOpts(ctx, conn, binary, args)
}

// ExecRemoteCommandWithOpts is a helper function for execing a command on a remote host with provided opts
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func ExecRemoteCommandWithOpts(ctx context.Context, conn *proxy.Conn, binary string, args []string, opts ...ExecRemoteOption) (*pb.ExecResponse, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("ExecRemoteCommand only supports single targets")
	}

	result, err := ExecRemoteCommandManyWithOpts(ctx, conn, binary, args)
	if err != nil {
		return nil, err
	}
	if len(result) < 1 {
		return nil, fmt.Errorf("ExecRemoteCommand error: received empty response")
	}
	if result[0].Error != nil {
		return nil, fmt.Errorf("ExecRemoteCommand error: %v", result[0].Error)
	}
	return result[0].Resp, nil
}

// ExecRemoteCommandMany is a helper function for execing a command on one or remote hosts
// using a proxy.Conn.
// `binary` refers to the absolute path of the binary file on the remote host.
//
// // Deprecated: Use ExecRemoteCommandManyWithOpts instead.
func ExecRemoteCommandMany(ctx context.Context, conn *proxy.Conn, binary string, args ...string) ([]*pb.RunManyResponse, error) {
	return ExecRemoteCommandManyWithOpts(ctx, conn, binary, args)
}

// ExecRemoteCommandManyWithOpts is a helper function for execing a command on one or remote hosts with provided opts
// using a proxy.Conn.
// `binary` refers to the absolute path of the binary file on the remote host.
func ExecRemoteCommandManyWithOpts(ctx context.Context, conn *proxy.Conn, binary string, args []string, opts ...ExecRemoteOption) ([]*pb.RunManyResponse, error) {
	execOptions := &execRemoteOptions{}
	for _, opt := range opts {
		opt(execOptions)
	}

	c := pb.NewExecClientProxy(conn)
	req := &pb.ExecRequest{
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
