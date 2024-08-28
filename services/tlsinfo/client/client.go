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
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/tlsinfo"
	"github.com/Snowflake-Labs/sansshell/services/util"
	cliUtils "github.com/Snowflake-Labs/sansshell/services/util/cli"
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
	serverName         string
	insecureSkipVerify bool
	printPEM           bool
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
func (c *getCertsCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.insecureSkipVerify, "insecure-skip-verify", false, "If true, will skip verification of server's certificate chain and host name")
	f.BoolVar(&c.printPEM, "pem", false, "Print certificates in PEM format")
	f.StringVar(&c.serverName, "server-name", "", "server-name is used to specify the Server Name Indication (SNI) during the TLS handshake. It allows client to indicate which hostname it's trying to connect to.")
}

func (c *getCertsCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Please specify server address.\n")
		return subcommands.ExitUsageError
	}

	proxy := pb.NewTLSInfoClientProxy(state.Conn)
	req := &pb.TLSCertificateRequest{
		ServerAddress:      f.Arg(0),
		InsecureSkipVerify: c.insecureSkipVerify,
		ServerName:         c.serverName,
	}
	respChan, err := proxy.GetTLSCertificateOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(state.Err[0], "failed to get TLS certificates: %v\n", err)
		return subcommands.ExitFailure
	}

	foundErr := false
	for r := range respChan {
		targetLogger := cliUtils.NewStyledCliLogger(state.Out[r.Index], state.Err[r.Index], &cliUtils.CliLoggerOptions{
			ApplyStylingForErr: util.IsStreamToTerminal(state.Err[r.Index]),
			ApplyStylingForOut: util.IsStreamToTerminal(state.Out[r.Index]),
		})

		if r.Error != nil {
			targetLogger.Errorf("Get TLS certificates failure: %v\n", r.Error)
			foundErr = true
			continue
		}
		for i, cert := range r.Resp.Certificates {
			if c.printPEM {
				if len(cert.Raw) == 0 {
					targetLogger.Errorf("no raw cert available for %v, sansshell-server may be too old to return cert info\n", cert.Subject)
				} else {
					err := pem.Encode(state.Out[r.Index], &pem.Block{
						Type:  "CERTIFICATE",
						Bytes: cert.Raw,
					})
					if err != nil {
						targetLogger.Errorf("unable to encode cert as pem: %v\n", cert.Raw)
					}
				}
			} else {
				targetLogger.Infof("---Server Certificate--- %d\n", i)
				targetLogger.Infof("Issuer: %v\n", cliUtils.Colorize(cliUtils.GreenText, cert.Issuer))
				targetLogger.Infof("Subject: %v\n", cliUtils.Colorize(cliUtils.GreenText, cert.Subject))
				targetLogger.Infof("NotBefore: %s\n", cliUtils.Colorize(cliUtils.GreenText, time.Unix(cert.NotBefore, 0)))
				targetLogger.Infof("NotAfter: %s\n", cliUtils.Colorize(cliUtils.GreenText, time.Unix(cert.NotAfter, 0)))
				targetLogger.Infof("DNS Names: %v\n", cliUtils.Colorize(cliUtils.GreenText, cert.DnsNames))
				targetLogger.Infof("IP Addresses: %v\n\n", cliUtils.Colorize(cliUtils.GreenText, cert.IpAddresses))
			}
		}
	}

	if foundErr {
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
