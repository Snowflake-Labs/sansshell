/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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

// Package client provides the client interface for 'tlsinfo'
package client

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/tlsinfo"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
)

const subPackage = "tlsinfo"

func init() {
	subcommands.Register(&tlsInfoCmd{}, subPackage)
}

type tlsInfoCmd struct{}

func (*tlsInfoCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&getCertsCmd{}, "")
	return c
}

func (*tlsInfoCmd) Name() string { return subPackage }
func (c *tlsInfoCmd) Synopsis() string {
	return client.GenerateSynopsis(c.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}

func (c *tlsInfoCmd) Usage() string {
	return client.GenerateUsage(subPackage, c.Synopsis())
}
func (*tlsInfoCmd) SetFlags(f *flag.FlagSet) {}

func (c *tlsInfoCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	p := c.GetSubpackage(f)
	return p.Execute(ctx, args...)
}

type getCertsCmd struct {
	serverAddr string
}

func (*getCertsCmd) Name() string { return "get-certs" }
func (c *getCertsCmd) Synopsis() string {
	return "Retrieve TLS Certificate of the specified TLS server"
}

func (c *getCertsCmd) Usage() string {
	return `get-certs serverAddr:
	Retrieve TLS Certificate of TLS server on serverAddr.
	serverAddr must be in the form of host:port.
	Example: get-certs example.com:443
	This is similar to "openssl s_client -showcerts -connect serverAddr"
	`
}
func (*getCertsCmd) SetFlags(f *flag.FlagSet) {}

func (c *getCertsCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Please speicfy server address.")
		return subcommands.ExitUsageError
	}

	proxy := pb.NewTLSInfoClientProxy(state.Conn)
	req := &pb.TLSCertificateRequest{
		ServerAddress: f.Arg(0),
	}
	respChan, err := proxy.GetTLSCertificateOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(state.Err[0], "failed to get TLS certificates: %v\n", err)
		return subcommands.ExitFailure
	}

	foundErr := false
	for r := range respChan {
		fmt.Fprintf(state.Out[r.Index], "Target %s result:\n", r.Target)
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Get TLS certificates failure: %v\n", r.Error)
			foundErr = true
			continue
		}
		for i, cert := range r.Resp.Certificates {
			fmt.Fprintf(state.Out[r.Index], "---Server Certificate--- %d\n", i)
			fmt.Fprintf(state.Out[r.Index], "Issuer: %v\n", cert.Issuer)
			fmt.Fprintf(state.Out[r.Index], "Subject: %v\n", cert.Subject)
			fmt.Fprintf(state.Out[r.Index], "NotBefore: %s\n", time.Unix(cert.NotBefore, 0))
			fmt.Fprintf(state.Out[r.Index], "NotAfter: %s\n", time.Unix(cert.NotAfter, 0))
			fmt.Fprintf(state.Out[r.Index], "DNS Names: %v\n", cert.DnsNames)
			fmt.Fprintf(state.Out[r.Index], "IP Addresses: %v\n\n", cert.IpAddresses)
		}
	}

	if foundErr {
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
