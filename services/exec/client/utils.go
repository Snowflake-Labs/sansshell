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

// ExecRemoteCommand is a helper function for execing a command on a remote host
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func ExecRemoteCommand(ctx context.Context, conn *proxy.Conn, binary string, args ...string) (*pb.ExecResponse, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("ExecRemoteCommand only supports single targets")
	}

	result, err := ExecRemoteCommandMany(ctx, conn, binary, args...)
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

// ExecRemoteCommandAs is a helper function for execing a command on a remote host as specified user
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func ExecRemoteCommandAs(ctx context.Context, conn *proxy.Conn, user string, binary string, args ...string) (*pb.ExecResponse, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("ExecRemoteCommand only supports single targets")
	}

	result, err := ExecRemoteCommandManyAs(ctx, conn, user, binary, args...)
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
func ExecRemoteCommandMany(ctx context.Context, conn *proxy.Conn, binary string, args ...string) ([]*pb.RunManyResponse, error) {
	c := pb.NewExecClientProxy(conn)
	req := &pb.ExecRequest{
		Command: binary,
		Args:    args,
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

// ExecRemoteCommandManyAs is a helper function for execing a command on one or remote hosts as specified user
// using a proxy.Conn.
// `binary` refers to the absolute path of the binary file on the remote host.
func ExecRemoteCommandManyAs(ctx context.Context, conn *proxy.Conn, user string, binary string, args ...string) ([]*pb.RunManyResponse, error) {
	c := pb.NewExecClientProxy(conn)
	req := &pb.ExecRequest{
		User:    user,
		Command: binary,
		Args:    args,
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
