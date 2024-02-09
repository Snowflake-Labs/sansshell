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

// Package client provides the client interface for 'tcpoverrpc'
package client

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/tcpoverrpc"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "tcpoverrpc"

func init() {
	subcommands.Register(&tcpCmd{}, subPackage)
}

func (*tcpCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&validateCmd{}, "")
	return c
}

type tcpCmd struct {
}

func (*tcpCmd) Name() string { return subPackage }
func (p *tcpCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *tcpCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (p *tcpCmd) SetFlags(f *flag.FlagSet) {
}

func (p *tcpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

type validateCmd struct {
	Hostname       string
	TimeoutSeconds int
}

func (*validateCmd) Name() string { return "validate" }
func (*validateCmd) Synopsis() string {
	return "Confirm TCP connectivity from target to a specified TCP server."
}
func (*validateCmd) Usage() string {
	return `tcpoverrpc validate [-hostname HOSTNAME] remoteport:
  Sends an empty TCP request and expects an empty response.  Only prints errors.
  Example:
  tcpoverrpc validate --hostname 10.1.23.4 443
`
}

func (p *validateCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.Hostname, "hostname", "localhost", "ip address or domain name to specify host")
	f.IntVar(&p.TimeoutSeconds, "timeout", 10, "maximum amount of time (in seconds) to wait for a TCP connect to complete.")
}

func (p *validateCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewTCPOverRPCClientProxy(state.Conn)
	port, err := strconv.Atoi(f.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Port could not be interpreted as a number.")
		return subcommands.ExitUsageError
	}
	if !util.ValidatePort(port) {
		fmt.Fprintln(os.Stderr, "Port could not be outside the range of [0~65535].")
		return subcommands.ExitUsageError
	}

	resp, err := c.OkOneMany(ctx, &pb.HostTCPRequest{Hostname: p.Hostname, Port: int32(port), TimeoutSeconds: int32(p.TimeoutSeconds)})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not send tcp validate request: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	hostnameAndPort := fmt.Sprintf("%s:%s", p.Hostname, f.Arg(1))

	retCode := subcommands.ExitSuccess
	for r := range resp {
		targetIP, _, err := net.SplitHostPort(r.Target)
		if err != nil {
			fmt.Printf("failed to split target host and port: %v", err)
		}
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "TCP validate from %s (index %d) to %s returned error: %v\n", targetIP, r.Index, hostnameAndPort, r.Error)
			// If any target had errors it needs to be reported for that target but we still
			// need to process responses off the channel. Final return code though should
			// indicate something failed.
			retCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprintf(state.Out[r.Index], "TCP validate from %s (index %d) to %s OK\n", targetIP, r.Index, hostnameAndPort)
	}
	return retCode
}
