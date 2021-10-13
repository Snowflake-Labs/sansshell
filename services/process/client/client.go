package client

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	pb "github.com/Snowflake-Labs/sansshell/services/process"
)

// A type for a custom flag for a list of ints in a comma separated list.
type intList []int64

// String implements as needed for flag.Value
func (i *intList) String() string {
	var out bytes.Buffer

	for _, t := range *i {
		out.WriteString(fmt.Sprintf("%d,", t))
	}
	o := out.String()
	// Trim last , off the end
	if len(o) > 0 {
		o = o[0 : len(o)-1]
	}
	return o
}

// Set implements parsing for int list flags as needed
// for flag.Value
func (i *intList) Set(val string) error {
	if len(*i) > 0 {
		return errors.New("intlist flag already set")
	}
	for _, t := range strings.Split(val, ",") {
		x, err := strconv.ParseInt(t, 0, 64)
		if err != nil {
			return fmt.Errorf("can't parse integer in list: %s", val)
		}
		*i = append(*i, x)
	}
	return nil
}

func init() {
	subcommands.Register(&processCmd{}, "raw")
}

type processCmd struct {
	pids intList
}

func (*processCmd) Name() string     { return "ps" }
func (*processCmd) Synopsis() string { return "Retrieve process list." }
func (*processCmd) Usage() string {
	return `ps:
  Read the process list from the remote machine.
`
}

func (p *processCmd) SetFlags(f *flag.FlagSet) {
	f.Var(&p.pids, "pids", "Restrict to only pids listed (separated by comma)")
}

func (p *processCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)

	c := pb.NewProcessClient(conn)

	req := &pb.ListRequest{}
	for _, p := range p.pids {
		req.Pids = append(req.Pids, p)
	}

	resp, err := c.List(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "List returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	fmtHeader := "%8s %8s %32s %4s %4s %8s %16s %20s %20s %8s %8s %8s %8s %8s %8s %8s %8s %5s %8s %5s %16s %16s %16s %16s %16s %16s %8s %s\n"
	fmtEntry := "%8d %8d %32s %4.1f %4.1f %8s %16s %20d %20d %8d %8d %8d %8d %8d %8d %8d %8d %5s %8x %5s %16x %16x %16x %16x %16x %16x %8d %s\n"
	fmt.Printf(fmtHeader, "PID", "PPID", "WCHAN", "%CPU", "%MEM", "START", "TIME", "RSS", "VSZ", "EGID", "EUID", "RGID", "RUID", "SGID", "SUID", "NICE", "PRIORITY", "CLS", "FLAG", "STAT", "EIP", "ESP", "BLOCKED", "CAUGHT", "IGNORED", "PENDING", "NLWP", "CMD")
	for _, p := range resp.ProcessEntries {
		var cls, stat string

		switch p.SchedulingClass {
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

		switch p.State {
		case pb.ProcessState_PROCESS_STATE_INTERRUPTIBLE_SLEEP:
			stat = "S"
		case pb.ProcessState_PROCESS_STATE_RUNNING:
			stat = "R"
		case pb.ProcessState_PROCESS_STATE_STOPPED_DEBUGGER:
			stat = "t"
		case pb.ProcessState_PROCESS_STATE_STOPPED_JOB_CONTROL:
			stat = "T"
		case pb.ProcessState_PROCESS_STATE_UNINTERRUPTIBLE_SLEEP:
			stat = "D"
		case pb.ProcessState_PROCESS_STATE_ZOMBIE:
			stat = "Z"
		default:
			stat = "?"
		}

		for _, s := range p.StateCode {
			switch s {
			case pb.ProcessStateCode_PROCESS_STATE_CODE_FOREGROUND_PGRP:
				stat += "+"
			case pb.ProcessStateCode_PROCESS_STATE_CODE_HIGH_PRIORITY:
				stat += "<"
			case pb.ProcessStateCode_PROCESS_STATE_CODE_LOCKED_PAGES:
				stat += "L"
			case pb.ProcessStateCode_PROCESS_STATE_CODE_LOW_PRIORITY:
				stat += "N"
			case pb.ProcessStateCode_PROCESS_STATE_CODE_SESSION_LEADER:
				stat += "s"
			case pb.ProcessStateCode_PROCESS_STATE_CODE_MULTI_THREADED:
				stat += "l"
			}
		}
		fmt.Printf(fmtEntry, p.Pid, p.Ppid, p.Wchan, p.CpuPercent, p.MemPercent, p.StartedTime, p.ElapsedTime, p.Rss, p.Vsize, p.Egid, p.Euid, p.Rgid, p.Ruid, p.Sgid, p.Suid, p.Nice, p.Priority, cls, p.Flags, stat, p.Eip, p.Esp, p.BlockedSignals, p.CaughtSignals, p.IgnoredSignals, p.PendingSignals, p.NumberOfThreads, p.Command)
	}

	return subcommands.ExitSuccess
}
