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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/subcommands"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	c.Register(&journalCmd{}, "")
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
	return `dmesg [--tail=N] [--grep=PATTERN] [-i] [-v]:
	 Print the messages from kernel ring buffer.
`
}

func (p *dmesgCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.grep, "grep", "", "regular expression filter")
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
			fmt.Fprintf(state.Out[i], "[%s]: %s", record.Time.AsTime().Local(), record.Message)
		}
	}
	return exit
}

type journalCmd struct {
	since   string
	until   string
	tail    int64
	unit    string
	explain bool
	output  string
}

func (*journalCmd) Name() string     { return "journalctl" }
func (*journalCmd) Synopsis() string { return "Get the log entries stored in journald" }
func (*journalCmd) Usage() string {
	return `journalCtl [--since|--S=X] [--until|-U=X] [-tail=X] [-u|-unit=X] [-o|--output=X] [-x] :
	Get the log entries stored in journald by systemd-journald.service 
`
}

func (p *journalCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.since, "since", "", "Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter from")
	f.StringVar(&p.since, "S", "", "Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter from (the date time is included)")
	f.StringVar(&p.until, "until", "", "Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter until (the date time is not included)")
	f.StringVar(&p.until, "U", "", "Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter until")
	f.StringVar(&p.unit, "unit", "", "Sets systemd unit to filter messages")
	f.StringVar(&p.output, "output", "", "Sets the format of the journal entries that will be shown. Right now only json and json-pretty are supported.")
	f.BoolVar(&p.explain, "x", false, "If true, augment log lines with explanatory texts from the message catalog.")
	f.Int64Var(&p.tail, "tail", 100, "If positive, the latest n records to fetch. By default, fetch latest 100 records. If negative, fetch all records")
}

func (p *journalCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewSysInfoClientProxy(state.Conn)

	// the output is case insensitive
	p.output = strings.ToLower(p.output)
	// currently output can only be json or json-pretty
	if p.output != "" && p.output != "json" && p.output != "json-pretty" {
		fmt.Fprintln(os.Stderr, "cannot set output to other formats unless json or json-pretty")
		return subcommands.ExitUsageError
	}

	req := &pb.JournalRequest{
		TailLine: int32(p.tail),
		Explain:  p.explain,
		Unit:     p.unit,
		Output:   p.output,
	}
	// Note: if timestamp is passed, don't forget to conver to UTC
	expectedTimeFormat := "2006-01-02 15:04:05"
	loc, err := time.LoadLocation("Local")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot get local location")
		return subcommands.ExitUsageError
	}
	if p.since != "" {
		sinceTime, err := time.ParseInLocation(expectedTimeFormat, p.since, loc)
		if err != nil {
			fmt.Fprintln(os.Stderr, "please specify correct time pattern YYYY-MM-DD HH:MM:SS")
			return subcommands.ExitUsageError
		}
		req.TimeSince = timestamppb.New(sinceTime.UTC())
	}

	if p.until != "" {
		untilTime, err := time.ParseInLocation(expectedTimeFormat, p.until, loc)
		if err != nil {
			fmt.Fprintln(os.Stderr, "please specify correct time pattern YYYY-MM-DD HH:MM:SS")
			return subcommands.ExitUsageError
		}
		req.TimeUntil = timestamppb.New(untilTime.UTC())
	}

	stream, err := c.JournalOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not info servers: %v\n", err)
		}
		return subcommands.ExitFailure
	}

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

			switch t := r.Resp.Response.(type) {
			case *pb.JournalReply_Journal:
				journal := t.Journal
				displayPid := ""
				if journal.Pid != 0 {

					displayPid = fmt.Sprintf("[%d]", journal.Pid)
				}
				fmt.Fprintf(state.Out[i], "[%s]  %s %s%s: %s\n", journal.RealtimeTimestamp.AsTime().Local(), journal.Hostname, journal.SyslogIdentifier, displayPid, journal.Message)
				// process explanatory texts if exists
				if journal.Catalog != "" {
					lines := strings.Split(journal.Catalog, "\n")
					// if last line is empty, just remove it
					if len(lines) > 0 && lines[len(lines)-1] == "" {
						lines = lines[:len(lines)-1]
					}
					for idx := range lines {
						lines[idx] = "-- " + lines[idx]
					}
					explainTexts := strings.Join(lines, "\n")
					fmt.Fprintf(state.Out[i], "%s\n", explainTexts)
				}
			case *pb.JournalReply_JournalRaw:
				journalRaw := t.JournalRaw
				// Encode the map to JSON
				var jsonData []byte
				if p.output == "json" {
					jsonData, err = json.Marshal(journalRaw.Entry)
					if err != nil {
						fmt.Fprintf(state.Err[r.Index], "Target %s (%d) returned cannot encode journal entry to JSON\n", r.Target, r.Index)
						exit = subcommands.ExitFailure
						continue
					}
				} else if p.output == "json-pretty" {
					jsonData, err = json.MarshalIndent(journalRaw.Entry, "", "        ")
					if err != nil {
						fmt.Fprintf(state.Err[r.Index], "Target %s (%d) returned cannot encode journal entry to pretty JSON\n", r.Target, r.Index)
						exit = subcommands.ExitFailure
						continue
					}
				}
				// Convert the JSON data to a string
				jsonString := string(jsonData)
				fmt.Fprintf(state.Out[i], "%s\n", jsonString)
			}
		}
	}
	return exit
}
