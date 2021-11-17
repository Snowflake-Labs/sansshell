package client

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/google/subcommands"

	pb "github.com/Snowflake-Labs/sansshell/services/process"
	"github.com/Snowflake-Labs/sansshell/services/util"
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
	subcommands.Register(&psCmd{}, "process")
	subcommands.Register(&pstackCmd{}, "process")
	subcommands.Register(&jstackCmd{}, "process")
	subcommands.Register(&dumpCmd{}, "process")
}

type psCmd struct {
	pids intList
}

func (*psCmd) Name() string     { return "ps" }
func (*psCmd) Synopsis() string { return "Retrieve process list." }
func (*psCmd) Usage() string {
	return `ps:
  Read the process list from the remote machine.
`
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
		fmt.Fprintf(os.Stderr, "ListOneMany returned error: %v\n", err)
		return subcommands.ExitFailure
	}
	for resp := range respChan {
		fmt.Fprintf(state.Out, "\nTarget: %s Entries: %d\n\n", resp.Target, len(resp.Resp.ProcessEntries))
		if resp.Error != nil {
			fmt.Fprintf(state.Out, "Got error from target %s - %v\n", resp.Target, resp.Error)
			continue
		}
		outputPsEntry(resp.Resp, state.Out)
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

type pstackCmd struct {
	pid int64
}

func (*pstackCmd) Name() string     { return "pstack" }
func (*pstackCmd) Synopsis() string { return "Retrieve stacks." }
func (*pstackCmd) Usage() string {
	return `pstack:
  Read the stacks for a given process id.
`
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
	c := pb.NewProcessClient(state.Conn)

	req := &pb.GetStacksRequest{
		Pid: p.pid,
	}

	resp, err := c.GetStacks(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetStacks returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	for _, s := range resp.Stacks {
		if s.ThreadNumber != 0 {
			fmt.Fprintf(os.Stdout, "Thread %d (Thread 0x%x (LWP %d)):\n", s.ThreadNumber, s.ThreadId, s.Lwp)
		}
		for _, t := range s.Stacks {
			fmt.Fprintln(os.Stdout, t)
		}
	}
	return subcommands.ExitSuccess
}

type jstackCmd struct {
	pid int64
}

func (*jstackCmd) Name() string     { return "jstack" }
func (*jstackCmd) Synopsis() string { return "Retrieve java stacks." }
func (*jstackCmd) Usage() string {
	return `jstack:
  Read the java stacks for a given process id.
`
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
	c := pb.NewProcessClient(state.Conn)

	req := &pb.GetJavaStacksRequest{
		Pid: p.pid,
	}

	resp, err := c.GetJavaStacks(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetJavaStacks returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	for _, s := range resp.Stacks {
		fmt.Fprintf(os.Stdout, "%q ", s.Name)
		if s.ThreadNumber != 0 {
			fmt.Fprintf(os.Stdout, "#%d ", s.ThreadNumber)
		}
		daemon := ""
		if s.Daemon {
			daemon = "daemon "
		}
		fmt.Fprintf(os.Stdout, "%s", daemon)
		if s.ThreadNumber != 0 {
			fmt.Fprintf(os.Stdout, "prio=%d ", s.Priority)
		}
		fmt.Fprintf(os.Stdout, "os_prio=%d cpu=%fms elapsed=%fs tid=0x%016x nid=0x%x %s ", s.OsPriority, s.CpuMs, s.ElapsedSec, s.ThreadId, s.NativeThreadId, s.State)
		if s.ThreadNumber != 0 {
			fmt.Fprintf(os.Stdout, "[0x%016x]\n", s.Pc)
		}
		for _, t := range s.Stacks {
			fmt.Fprintln(os.Stdout, t)
		}
		fmt.Fprintln(os.Stdout)
	}
	return subcommands.ExitSuccess
}

// createOutput is a helper func for gcore/jmap which creates the output file or writes
// to stderr. It will print an error so calling code should simply return.
func createOutput(path string) (*os.File, error) {
	var output *os.File
	var err error
	if path == "-" {
		output = os.Stdout
	} else {
		output, err = os.Create(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't create output file %s: %v\n", path, err)
			return nil, err
		}
	}
	return output, nil
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
	return `dump:
  Generate a memory dump for a given process id.
`
}

func (p *dumpCmd) SetFlags(f *flag.FlagSet) {
	f.Int64Var(&p.pid, "pid", 0, "Process to generate a core dump against.")
	f.StringVar(&p.dumpType, "dump-type", "GCORE", fmt.Sprintf("Dump type to use(one of: [%s])", strings.Join(shortDumpTypeNames(), ",")))
	f.StringVar(&p.output, "output", "", `Output to write data. - indicates to use stdout. A normal file path applies.

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

	if p.output == "" {
		fmt.Fprintln(os.Stderr, "--output must be specified")
		return subcommands.ExitFailure
	}
	state := args[0].(*util.ExecuteState)
	c := pb.NewProcessClient(state.Conn)

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

	stream, err := c.GetMemoryDump(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetMemoryDump returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	var output *os.File
	if req.GetUrl() == nil {
		output, err = createOutput(p.output)
		if err != nil {
			return subcommands.ExitFailure
		}
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Receive error: %v\n", err)
			return subcommands.ExitFailure
		}
		if output != nil {
			n, err := output.Write(resp.Data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to %s. Only wrote %d bytes, expected %d - %v\n", p.output, n, len(resp.Data), err)
				return subcommands.ExitFailure
			}
		}
	}
	return subcommands.ExitSuccess
}
