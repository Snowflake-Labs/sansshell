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
	"google.golang.org/protobuf/types/known/durationpb"
	"io"
	"os"
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
	timeout     time.Duration
}

func (*dmesgCmd) Name() string     { return "dmesg" }
func (*dmesgCmd) Synopsis() string { return "View the messages in kernel ring buffer" }
func (*dmesgCmd) Usage() string {
	return `dmesg [--tail=N] [--grep=PATTERN] [-i] [-v] [--timeout=N]:
	 Print the messages from kernel ring buffer.
`
}

func (p *dmesgCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.grep, "grep", "", "regular expression filter")
	f.Int64Var(&p.tail, "tail", -1, "tail the latest n lines")
	f.BoolVar(&p.ignoreCase, "i", false, "ignore case")
	f.BoolVar(&p.invertMatch, "v", false, "invert match")
	f.DurationVar(&p.timeout, "dmesg-read-timeout", 2*time.Second, "timeout collection of messages after specified duration, default is 2s, max 30s")
}

func (p *dmesgCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewSysInfoClientProxy(state.Conn)

	if p.timeout > pb.MaxDmesgTimeout {
		fmt.Printf("Invalid dmesg-read-timeout, value %s is higher than maximum dmesg-read-timeout of %s\n", p.timeout.String(), pb.MaxDmesgTimeout.String())
		return subcommands.ExitUsageError
	}
	if p.timeout < pb.MinDmesgTimeout {
		fmt.Printf("Invalid dmesg-read-timeout, value %s is lower than minimum value of %s\n", p.timeout.String(), pb.MinDmesgTimeout.String())
		return subcommands.ExitUsageError
	}

	req := &pb.DmesgRequest{
		TailLines:   int32(p.tail),
		Grep:        p.grep,
		IgnoreCase:  p.ignoreCase,
		InvertMatch: p.invertMatch,
		Timeout:     durationpb.New(p.timeout),
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
	since      string
	until      string
	tail       uint64
	unit       string
	enableJSON bool
}

func (*journalCmd) Name() string     { return "journalctl" }
func (*journalCmd) Synopsis() string { return "Get the log entries stored in journald" }
func (*journalCmd) Usage() string {
	return `journalctl [--since|--S=X] [--until|-U=X] [-tail=X] [-u|-unit=X]:
	Get the log entries stored in journald by systemd-journald.service
`
}

func (p *journalCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.since, "since", "", "Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter from")
	f.StringVar(&p.since, "S", "", "Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter from (the date time is included)")
	f.StringVar(&p.until, "until", "", "Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter until (the date time is not included)")
	f.StringVar(&p.until, "U", "", "Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter until")
	f.StringVar(&p.unit, "unit", "", "Sets systemd unit to filter messages")
	f.StringVar(&p.unit, "u", "", "Sets systemd unit to filter messages")
	f.BoolVar(&p.enableJSON, "json", false, "Print the journal entries in JSON format(can work with jq for better visualization)")
	f.Uint64Var(&p.tail, "tail", 100, "If positive, the latest n records to fetch. By default, fetch latest 100 records. The upper limit is 10000 for now")
}

func (p *journalCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewSysInfoClientProxy(state.Conn)

	// validate the tail number is set correctly
	// reason we set this limit is due to the limit of DefRunBufLimit set for stdout
	// https://github.com/Snowflake-Labs/sansshell/blob/989cb789586532eaa22b75303ea94c97ca246306/services/util/util.go#L161.
	if p.tail > pb.JounalEntriesLimit {
		fmt.Fprintln(os.Stderr, "cannot set tail number larger than 10000")
		return subcommands.ExitUsageError
	}

	req := &pb.JournalRequest{
		TailLine:   uint32(p.tail),
		Unit:       p.unit,
		EnableJson: p.enableJSON,
	}
	// Note: if timestamp is passed, don't forget to conver to UTC before sending the rpc request
	loc, err := time.LoadLocation("Local")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot get local location")
		return subcommands.ExitUsageError
	}
	if p.since != "" {
		sinceTime, err := time.ParseInLocation(pb.TimeFormat_YYYYMMDDHHMMSS, p.since, loc)
		if err != nil {
			fmt.Fprintln(os.Stderr, "please specify correct time pattern YYYY-MM-DD HH:MM:SS")
			return subcommands.ExitUsageError
		}
		req.TimeSince = timestamppb.New(sinceTime.UTC())
	}

	if p.until != "" {
		untilTime, err := time.ParseInLocation(pb.TimeFormat_YYYYMMDDHHMMSS, p.until, loc)
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
		for _, r := range resp {
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

			// format the output in different way based on the given return type
			switch t := r.Resp.Response.(type) {
			case *pb.JournalReply_Journal:
				journal := t.Journal
				// some journal entries may not be generated by a process(no pid)
				displayPid := ""
				if journal.Pid != 0 {
					displayPid = fmt.Sprintf("[%d]", journal.Pid)
				}
				fmt.Fprintf(state.Out[r.Index], "[%s]  %s %s%s: %s\n", journal.RealtimeTimestamp.AsTime().Local(), journal.Hostname, journal.SyslogIdentifier, displayPid, journal.Message)
			case *pb.JournalReply_JournalRaw:
				journalRaw := t.JournalRaw
				// Encode the map to JSON
				var jsonData []byte
				if p.enableJSON {
					jsonData, err = json.Marshal(journalRaw.Entry)
					if err != nil {
						fmt.Fprintf(state.Err[r.Index], "Target %s (%d) returned cannot encode journal entry to JSON\n", r.Target, r.Index)
						exit = subcommands.ExitFailure
						continue
					}
				}
				// Convert the JSON data to a string
				jsonString := string(jsonData)
				fmt.Fprintf(state.Out[r.Index], "%s\n", jsonString)
			}
		}
	}
	return exit
}
