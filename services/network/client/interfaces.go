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

package client

import (
	"context"
	"flag"
	"fmt"
	"net"

	pb "github.com/Snowflake-Labs/sansshell/services/network"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
	"google.golang.org/protobuf/types/known/emptypb"
)

type listInterfacesCmd struct {
	namesOnly bool
}

func (*listInterfacesCmd) Name() string     { return "interfaces" }
func (*listInterfacesCmd) Synopsis() string { return "List available network interfaces" }
func (*listInterfacesCmd) Usage() string {
	return `interfaces [-N]
	 List network interfaces
`
}

func (p *listInterfacesCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.namesOnly, "N", false, "display only interface names")
}

func (p *listInterfacesCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	const (
		listingHeaderFormat = "%-3s %-16s %-20s %6s  %s"
		listingItemFormat   = "%-3d %-16s %-20s %6d  %s"
	)

	state := args[0].(*util.ExecuteState)
	c := pb.NewPacketCaptureClientProxy(state.Conn)

	resp, err := c.ListInterfacesOneMany(ctx, &emptypb.Empty{})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not info servers: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "info for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			// If any target had errors it needs to be reported for that target but we still
			// need to process responses off the channel. Final return code though should
			// indicate something failed.
			retCode = subcommands.ExitFailure
			continue
		}
		if !p.namesOnly {
			fmt.Fprintf(state.Out[r.Index], listingHeaderFormat+"\n", "idx", "name", "hwaddr", "MTU", "flags")
		}
		for _, iface := range r.Resp.Interfaces {
			var s string
			if p.namesOnly {
				s = iface.Name
			} else {
				s = fmt.Sprintf(listingItemFormat, iface.Index, iface.Name, net.HardwareAddr(iface.Hwaddr).String(), iface.Mtu, net.Flags(iface.Flags).String())
			}
			fmt.Fprintln(state.Out[r.Index], s)
		}
	}
	return retCode
}
