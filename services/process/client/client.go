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

// Package client provides the client interface for 'process'
package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/process"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "process"

func init() {
	subcommands.Register(&processCmd{}, subPackage)
}

func (*processCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&dumpCmd{}, "")
	c.Register(&jstackCmd{}, "")
	c.Register(&killCmd{}, "")
	c.Register(&psCmd{}, "")
	c.Register(&pstackCmd{}, "")
	return c
}

type processCmd struct{}

func (*processCmd) Name() string { return subPackage }
func (p *processCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *processCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*processCmd) SetFlags(f *flag.FlagSet) {}

func (p *processCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

func outputEntryHeader(out io.Writer, target string, index int) {
	fmt.Fprintf(out, "\nTarget: %s Index: %d \n\n", target, index)
}

type psCmd struct {
	pids util.IntSliceFlags
}

func (*psCmd) Name() string     { return "ps" }
func (*psCmd) Synopsis() string { return "Retrieve process list." }
func (*psCmd) Usage() string {
	return "ps: Read the process list from the remote machine.\n"
}

func (p *psCmd) SetFlags(f *flag.FlagSet) {
	f.Var(&p.pids, "pids", "Restrict to only pids listed (separated by comma)")
}

func (p *psCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)

	c := pb.NewProcessClientProxy(state.Conn)
	req := &pb.ListRequest{}
	for _, pid := range p.pids {
		req.Pids = append(req.Pids, pid)
	}

	respChan, err := c.ListOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - List returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}
	for resp := range respChan {
		if resp.Error != nil {
			fmt.Fprintf(state.Err[resp.Index], "Got error from target %s (%d) - %v\n", resp.Target, resp.Index, resp.Error)
			continue
		}
		outputEntryHeader(state.Out[resp.Index], resp.Target, resp.Index)
		outputPsEntry(resp.Resp, state.Out[resp.Index])
	}
	return subcommands.ExitSuccess
}

func outputPsEntry(resp *pb.ListReply, out io.Writer) {
	fmtHeader := "%8s %8s %32s %4s %4s %8s %16s %20s %20s %8s %8s %8s %8s %8s %8s %8s %8s %5s %8s %5s %16s %16s %16s %16s %16s %16s %8s %s\n"
	fmtEntry := "%8d %8d %32s %4.1f %4.1f %8s %16s %20d %20d %8d %8d %8d %8d %8d %8d %8s %8d %5s %8x %5s %16x %16x %16x %16x %16x %16x %8d %s\n"
	fmt.Fprintf(out, fmtHeader, "PID", "PPID", "WCHAN", "%CPU", "%MEM", "START", "TIME", "RSS", "VSZ", "EGID", "EUID", "RGID", "RUID", "SGID", "SUID", "NICE", "PRIORITY", "CLS", "FLAG", "STAT", "EIP", "ESP", "BLOCKED", "CAUGHT", "IGNORED", "PENDING", "NLWP", "CMD")

	for _, entry := range resp.ProcessEntries {
		cls := parseClass(entry.SchedulingClass)
		stat := parseState(entry.State, entry.StateCode)

		nice := fmt.Sprintf("%d", entry.Nice)
		// These scheduling classes are linux real time and nice doesn't apply.
		if cls == "RR" || cls == "FF" {
			nice = "-"
		}

		// Print everything from this entry.
		fmt.Fprintf(out, fmtEntry, entry.Pid, entry.Ppid, entry.Wchan, entry.CpuPercent, entry.MemPercent, entry.StartedTime, entry.ElapsedTime, entry.Rss, entry.Vsize, entry.Egid, entry.Euid, entry.Rgid, entry.Ruid, entry.Sgid, entry.Suid, nice, entry.Priority, cls, entry.Flags, stat, entry.Eip, entry.Esp, entry.BlockedSignals, entry.CaughtSignals, entry.IgnoredSignals, entry.PendingSignals, entry.NumberOfThreads, entry.Command)
	}
}

func parseClass(schedulingClass pb.SchedulingClass) string {
	var cls string

	switch schedulingClass {
	case pb.SchedulingClass_SCHEDULING_CLASS_BATCH:
		cls = "B"
	case pb.SchedulingClass_SCHEDULING_CLASS_DEADLINE:
		cls = "DLN"
	case pb.SchedulingClass_SCHEDULING_CLASS_FIFO:
		cls = "FF"
	case pb.SchedulingClass_SCHEDULING_CLASS_IDLE:
		cls = "IDL"
	case pb.SchedulingClass_SCHEDULING_CLASS_ISO:
		cls = "ISO"
	case pb.SchedulingClass_SCHEDULING_CLASS_NOT_REPORTED:
		cls = "-"
	case pb.SchedulingClass_SCHEDULING_CLASS_OTHER:
		cls = "TS"
	case pb.SchedulingClass_SCHEDULING_CLASS_RR:
		cls = "RR"
	default:
		cls = "?"
	}
	return cls
}

func parseState(processState pb.ProcessState, codes []pb.ProcessStateCode) string {
	var state string

	switch processState {
	case pb.ProcessState_PROCESS_STATE_INTERRUPTIBLE_SLEEP:
		state = "S"
	case pb.ProcessState_PROCESS_STATE_RUNNING:
		state = "R"
	case pb.ProcessState_PROCESS_STATE_STOPPED_DEBUGGER:
		state = "t"
	case pb.ProcessState_PROCESS_STATE_STOPPED_JOB_CONTROL:
		state = "T"
	case pb.ProcessState_PROCESS_STATE_UNINTERRUPTIBLE_SLEEP:
		state = "D"
	case pb.ProcessState_PROCESS_STATE_ZOMBIE:
		state = "Z"
	default:
		state = "?"
	}

	for _, s := range codes {
		switch s {
		case pb.ProcessStateCode_PROCESS_STATE_CODE_FOREGROUND_PGRP:
			state += "+"
		case pb.ProcessStateCode_PROCESS_STATE_CODE_HIGH_PRIORITY:
			state += "<"
		case pb.ProcessStateCode_PROCESS_STATE_CODE_LOCKED_PAGES:
			state += "L"
		case pb.ProcessStateCode_PROCESS_STATE_CODE_LOW_PRIORITY:
			state += "N"
		case pb.ProcessStateCode_PROCESS_STATE_CODE_SESSION_LEADER:
			state += "s"
		case pb.ProcessStateCode_PROCESS_STATE_CODE_MULTI_THREADED:
			state += "l"
		}
	}
	return state
}

type killCmd struct {
	pid    uint64
	signal uint
}

func (*killCmd) Name() string     { return "kill" }
func (*killCmd) Synopsis() string { return "Send a signal to a process id." }
func (*killCmd) Usage() string {
	return "kill: Send a signal to a process id.\n"
}

func (p *killCmd) SetFlags(f *flag.FlagSet) {
	f.Uint64Var(&p.pid, "pid", 0, "Process ID to send signal")
	f.UintVar(&p.signal, "signal", 0, "Signal to send process ID")
}

func (p *killCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)

	c := pb.NewProcessClientProxy(state.Conn)
	req := &pb.KillRequest{
		Pid:    p.pid,
		Signal: uint32(p.signal),
	}

	respChan, err := c.KillOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - Kill returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	for resp := range respChan {
		if resp.Error != nil {
			fmt.Fprintf(state.Err[resp.Index], "Got error from target %s (%d) - %v\n", resp.Target, resp.Index, resp.Error)
		}
	}
	return subcommands.ExitSuccess
}

type pstackCmd struct {
	pid int64
}

func (*pstackCmd) Name() string     { return "pstack" }
func (*pstackCmd) Synopsis() string { return "Retrieve stacks." }
func (*pstackCmd) Usage() string {
	return "pstack: Read the stacks for a given process id.\n"
}

func (p *pstackCmd) SetFlags(f *flag.FlagSet) {
	f.Int64Var(&p.pid, "pid", 0, "Process to execute pstack against.")
}

func (p *pstackCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if p.pid <= 0 {
		fmt.Fprintln(os.Stderr, "--pid must be specified")
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewProcessClientProxy(state.Conn)

	req := &pb.GetStacksRequest{
		Pid: p.pid,
	}

	respChan, err := c.GetStacksOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - GetStacks returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for resp := range respChan {
		if resp.Error != nil {
			fmt.Fprintf(state.Err[resp.Index], "Got error from target %s (%d) - %v\n", resp.Target, resp.Index, resp.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		outputEntryHeader(state.Out[resp.Index], resp.Target, resp.Index)
		for _, s := range resp.Resp.Stacks {
			if s.ThreadNumber != 0 {
				fmt.Fprintf(state.Out[resp.Index], "Thread %d (Thread 0x%x (LWP %d)):\n", s.ThreadNumber, s.ThreadId, s.Lwp)
			}
			for _, t := range s.Stacks {
				fmt.Fprintln(state.Out[resp.Index], t)
			}
		}
	}
	return retCode
}

type jstackCmd struct {
	pid int64
}

func (*jstackCmd) Name() string     { return "jstack" }
func (*jstackCmd) Synopsis() string { return "Retrieve java stacks." }
func (*jstackCmd) Usage() string {
	return "jstack: Read the java stacks for a given process id.\n"
}

func (p *jstackCmd) SetFlags(f *flag.FlagSet) {
	f.Int64Var(&p.pid, "pid", 0, "Process to execute pstack against.")
}

func (p *jstackCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if p.pid <= 0 {
		fmt.Fprintln(os.Stderr, "--pid must be specified")
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewProcessClientProxy(state.Conn)

	req := &pb.GetJavaStacksRequest{
		Pid: p.pid,
	}

	respChan, err := c.GetJavaStacksOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - GetJavaStacks returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for resp := range respChan {
		if resp.Error != nil {
			fmt.Fprintf(state.Err[resp.Index], "Got error from target %s (%d) - %v\n", resp.Target, resp.Index, resp.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		outputEntryHeader(state.Out[resp.Index], resp.Target, resp.Index)
		for _, s := range resp.Resp.Stacks {
			fmt.Fprintf(state.Out[resp.Index], "%q ", s.Name)
			if s.ThreadNumber != 0 {
				fmt.Fprintf(state.Out[resp.Index], "#%d ", s.ThreadNumber)
			}
			daemon := ""
			if s.Daemon {
				daemon = "daemon "
			}
			fmt.Fprintf(state.Out[resp.Index], "%s", daemon)
			if s.ThreadNumber != 0 {
				fmt.Fprintf(state.Out[resp.Index], "prio=%d ", s.Priority)
			}
			fmt.Fprintf(state.Out[resp.Index], "os_prio=%d cpu=%fms elapsed=%fs tid=0x%016x nid=0x%x %s ", s.OsPriority, s.CpuMs, s.ElapsedSec, s.ThreadId, s.NativeThreadId, s.State)
			if s.ThreadNumber != 0 {
				fmt.Fprintf(state.Out[resp.Index], "[0x%016x]\n", s.Pc)
			}
			for _, t := range s.Stacks {
				fmt.Fprintln(state.Out[resp.Index], t)
			}
			fmt.Fprintln(state.Out[resp.Index])
		}
	}
	return retCode
}

func flagToType(val string) (pb.DumpType, error) {
	v := fmt.Sprintf("DUMP_TYPE_%s", strings.ToUpper(val))
	i, ok := pb.DumpType_value[v]
	if !ok {
		return pb.DumpType_DUMP_TYPE_UNKNOWN, fmt.Errorf("no such sumtype value: %s", v)
	}
	return pb.DumpType(i), nil
}

func shortDumpTypeNames() []string {
	var shortNames []string
	for k := range pb.DumpType_value {
		if k != "DUMP_TYPE_UNKNOWN" {
			shortNames = append(shortNames, strings.TrimPrefix(k, "DUMP_TYPE_"))
		}
	}
	sort.Strings(shortNames)
	return shortNames
}

type dumpCmd struct {
	pid      int64
	dumpType string
	output   string
}

func (*dumpCmd) Name() string     { return "dump" }
func (*dumpCmd) Synopsis() string { return "Create a memory dump of a running process." }
func (*dumpCmd) Usage() string {
	return "dump: Generate a memory dump for a given process id."
}

func (p *dumpCmd) SetFlags(f *flag.FlagSet) {
	f.Int64Var(&p.pid, "pid", 0, "Process to generate a core dump against.")
	f.StringVar(&p.dumpType, "dump-type", "GCORE", fmt.Sprintf("Dump type to use(one of: [%s])", strings.Join(shortDumpTypeNames(), ",")))
	f.StringVar(&p.output, "output", "", `Output to write data remotely. Leave blank and --outputs will be used for local destinations.

This will also accept URL options of the form:

	s3://bucket (AWS)
	azblob://bucket (Azure)
	gs://bucket (GCP)
	
	See https://gocloud.dev/howto/blob/ for details on options.`)
}

var validOutputPrefixes = []string{
	"s3://",
	"azblob://",
	"gs://",
}

func (p *dumpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	dt, err := flagToType(p.dumpType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse dump type --dump-type: %s invalid\n", p.dumpType)
		return subcommands.ExitFailure
	}
	if p.pid <= 0 {
		fmt.Fprintln(os.Stderr, "--pid must be specified")
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewProcessClientProxy(state.Conn)

	req := &pb.GetMemoryDumpRequest{
		Pid:         p.pid,
		DumpType:    dt,
		Destination: &pb.GetMemoryDumpRequest_Stream{},
	}

	for _, pre := range validOutputPrefixes {
		if strings.HasPrefix(p.output, pre) {
			req.Destination = &pb.GetMemoryDumpRequest_Url{
				Url: &pb.DumpDestinationUrl{
					Url: p.output,
				},
			}
			break
		}
	}

	stream, err := c.GetMemoryDumpOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - GetMemoryDump returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	targetsDone := make(map[int]bool)
	retCode := subcommands.ExitSuccess
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		// If the stream returns an error we're just done.
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			// But...we only do this for targets that aren't complete. A complete target
			// didn't have an error. i.e. we got N done then the context expired.
			for i, e := range state.Err {
				if !targetsDone[i] {
					fmt.Fprintf(e, "Receive error: %v\n", err)
				}
			}
			retCode = subcommands.ExitFailure
			break
		}
		// Even if we're not writing output we have to process all responses to check for errors.
		for _, r := range resp {
			if r.Error != nil && r.Error != io.EOF {
				fmt.Fprintf(state.Err[r.Index], "Error for target %s (%d): %v\n", r.Target, r.Index, r.Error)
				retCode = subcommands.ExitFailure
				continue
			}

			// At EOF this target is done.
			if r.Error == io.EOF {
				targetsDone[r.Index] = true
				continue
			}

			if p.output == "" {
				n, err := state.Out[r.Index].Write(r.Resp.Data)
				if err != nil {
					fmt.Fprintf(state.Err[r.Index], "Error writing to %s. Only wrote %d bytes, expected %d - %v\n", p.output, n, len(r.Resp.Data), err)
					return subcommands.ExitFailure
				}
			}
		}
	}
	return retCode
}
