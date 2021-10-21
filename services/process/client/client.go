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
	subcommands.Register(&processCmd{}, "process")
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
	for _, pid := range p.pids {
		req.Pids = append(req.Pids, pid)
	}

	resp, err := c.List(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "List returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	fmtHeader := "%8s %8s %32s %4s %4s %8s %16s %20s %20s %8s %8s %8s %8s %8s %8s %8s %8s %5s %8s %5s %16s %16s %16s %16s %16s %16s %8s %s\n"
	fmtEntry := "%8d %8d %32s %4.1f %4.1f %8s %16s %20d %20d %8d %8d %8d %8d %8d %8d %8s %8d %5s %8x %5s %16x %16x %16x %16x %16x %16x %8d %s\n"
	fmt.Printf(fmtHeader, "PID", "PPID", "WCHAN", "%CPU", "%MEM", "START", "TIME", "RSS", "VSZ", "EGID", "EUID", "RGID", "RUID", "SGID", "SUID", "NICE", "PRIORITY", "CLS", "FLAG", "STAT", "EIP", "ESP", "BLOCKED", "CAUGHT", "IGNORED", "PENDING", "NLWP", "CMD")

	for _, entry := range resp.ProcessEntries {
		cls := parseClass(entry.SchedulingClass)
		stat := parseState(entry.State, entry.StateCode)

		nice := fmt.Sprintf("%d", entry.Nice)
		// These scheduling classes are linux real time and nice doesn't apply.
		if cls == "RR" || cls == "FF" {
			nice = "-"
		}

		// Print everything from this entry.
		fmt.Printf(fmtEntry, entry.Pid, entry.Ppid, entry.Wchan, entry.CpuPercent, entry.MemPercent, entry.StartedTime, entry.ElapsedTime, entry.Rss, entry.Vsize, entry.Egid, entry.Euid, entry.Rgid, entry.Ruid, entry.Sgid, entry.Suid, nice, entry.Priority, cls, entry.Flags, stat, entry.Eip, entry.Esp, entry.BlockedSignals, entry.CaughtSignals, entry.IgnoredSignals, entry.PendingSignals, entry.NumberOfThreads, entry.Command)
	}

	return subcommands.ExitSuccess
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
