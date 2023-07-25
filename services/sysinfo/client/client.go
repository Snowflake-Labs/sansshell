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

// Package client provides the client interface for 'sysinfo'
package client

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/google/subcommands"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/sysinfo"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "sysinfo"

func init() {
	subcommands.Register(&sysinfoCmd{}, subPackage)
}

func (*sysinfoCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&uptimeCmd{}, "")
	c.Register(&dmesgCmd{}, "")
	return c
}

type sysinfoCmd struct{}

func (*sysinfoCmd) Name() string { return subPackage }
func (p *sysinfoCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *sysinfoCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*sysinfoCmd) SetFlags(f *flag.FlagSet) {}

func (p *sysinfoCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

type uptimeCmd struct{}

func (*uptimeCmd) Name() string     { return "uptime" }
func (*uptimeCmd) Synopsis() string { return "Get the uptime of the system" }
func (*uptimeCmd) Usage() string {
	return `uptime :
	 Print the uptime of the system in below format: System_idx (ip:port) up for X days, X hours, X minutes, X seconds | total X seconds (X is a placeholder that will be replaced)
`
}

func (p *uptimeCmd) SetFlags(f *flag.FlagSet) {}

func (p *uptimeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewSysInfoClientProxy(state.Conn)

	resp, err := c.UptimeOneMany(ctx, &emptypb.Empty{})
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
		d := r.Resp.UptimeSeconds.AsDuration()
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		seconds := int(d.Seconds()) % 60
		days := hours / 24
		hours = hours % 24
		fmt.Fprintf(state.Out[r.Index], "Target %s (%d) up for %d days, %d hours, %d minutes, %d seconds | total %.f seconds \n", r.Target, r.Index, days, hours, minutes, seconds, d.Seconds())
	}
	return retCode
}

type dmesgCmd struct {
	tail        int64
	grep        string
	ignoreCase  bool
	invertMatch bool
}

func (*dmesgCmd) Name() string     { return "dmesg" }
func (*dmesgCmd) Synopsis() string { return "View the messages in kernel ring buffer" }
func (*dmesgCmd) Usage() string {
	return `demsg [--tail=N] [--grep=PATTERN] [-i] [-v]:
	 Print the messages from kernel ring buffer.
`
}

func (p *dmesgCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.grep, "grep", "", "regual expression filter")
	f.Int64Var(&p.tail, "tail", -1, "tail the latest n lines")
	f.BoolVar(&p.ignoreCase, "i", false, "ignore case")
	f.BoolVar(&p.invertMatch, "v", false, "invert match")
}

func (p *dmesgCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewSysInfoClientProxy(state.Conn)

	req := &pb.DmesgRequest{
		TailLines:   int32(p.tail),
		Grep:        p.grep,
		IgnoreCase:  p.ignoreCase,
		InvertMatch: p.invertMatch,
	}
	stream, err := c.DmesgOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not info servers: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	// printed timestamp will be the following format: date time timezone
	// date: YYYY-MM-DD
	// time: HH-MM-SS
	// timezone: MST/PDT etc.
	timestampFormat := "2006-01-02 15:04:05 MST"
	targetsDone := make(map[int]bool)
	exit := subcommands.ExitSuccess
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			// But...we only do this for targets that aren't complete. A complete target
			// didn't have an error. i.e. we got N done then the context expired.
			for i, e := range state.Err {
				if !targetsDone[i] {
					fmt.Fprintf(e, "Stream error: %v\n", err)
				}
			}
			exit = subcommands.ExitFailure
			break
		}
		for i, r := range resp {
			if r.Error != nil && r.Error != io.EOF {
				fmt.Fprintf(state.Err[r.Index], "Target %s (%d) returned error - %v\n", r.Target, r.Index, r.Error)
				targetsDone[r.Index] = true
				// If any target had errors it needs to be reported for that target but we still
				// need to process responses off the channel. Final return code though should
				// indicate something failed.
				exit = subcommands.ExitFailure
				continue
			}
			// At EOF this target is done.
			if r.Error == io.EOF {
				targetsDone[r.Index] = true
				continue
			}
			record := r.Resp.Record
			fmt.Fprintf(state.Out[i], "[%s]: %s", record.Time.AsTime().Local().Format(timestampFormat), record.Message)
		}
	}
	return exit
}
