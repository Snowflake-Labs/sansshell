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

package client

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const subPackage = "fdb"

func init() {
	subcommands.Register(&fdbCmd{}, subPackage)
}

// funcValue is a flag.Value that implements IsBoolFlag.
// Used below for boolean flag processing using flag.Func
type funcValue func(string) error

func (f funcValue) Set(s string) error { return f(s) }
func (f funcValue) String() string     { return "" }
func (f funcValue) IsBoolFlag() bool   { return true }

// processLog does setup and opens up the logfile we'll be appending to based on the passed pb.Log struct.
func processLog(state *util.ExecuteState, index int, log *pb.Log, openFiles map[string]*os.File) (subcommands.ExitStatus, map[string]*os.File) {
	// If it's already open we don't need to do anything
	if openFiles[log.Filename] != nil {
		return subcommands.ExitSuccess, openFiles
	}

	// The server returns a full path where-as locally we drop into the directory state says to use.
	// This means we can only depend on the basename of the path which may have duplicates.
	fn := filepath.Base(log.Filename)
	if fn == "" {
		fmt.Fprintf(state.Err[index], "RPC returned an invalid log structure: %+v\n", log)
		return subcommands.ExitFailure, openFiles
	}

	// See if the file already exists. If so we'll keep trying until we can append a number onto the name.
	// Create index based names for each log.
	fn = fmt.Sprintf("%d.%s", index, fn)
	path := filepath.Join(state.Dir, fn)

	// If the stat succeeds it already exists so we need to find another filename.
	if _, err := os.Stat(path); err == nil {
		i := 1
		for {
			// If the same basename exists we'll try and
			// create a new one with -NUMBER on the end.
			// Start at one until we find a unique one.
			p := fmt.Sprintf("%s-%d", path, i)
			if _, err := os.Stat(p); err != nil {
				path = p
				break
			}
			i++
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(state.Err[index], "can't write logfile %s: %v\n", path, err)
		return subcommands.ExitFailure, openFiles
	}
	openFiles[log.Filename] = f

	return subcommands.ExitSuccess, openFiles
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&fdbCLICmd{}, "")
	c.Register(&fdbConfCmd{}, "")

	return c
}

type fdbCmd struct{}

func (*fdbCmd) Name() string             { return subPackage }
func (*fdbCmd) SetFlags(_ *flag.FlagSet) {}
func (*fdbCmd) Synopsis() string {
	return client.GenerateSynopsis(setup(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *fdbCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}

func (p *fdbCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setup(f)
	return c.Execute(ctx, args...)
}

const fdbCLIPackage = "fdbcli"

func setupFDBCLI(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(fdbCLIPackage, f)
	c.Register(&fdbCLIAdvanceversionCmd{}, "")
	c.Register(&fdbCLIBeginCmd{}, "")
	c.Register(&fdbCLIBlobrangeCmd{}, "")
	c.Register(&fdbCLICacheRangeCmd{}, "")
	c.Register(&fdbCLIChangefeedCmd{}, "")
	c.Register(&fdbCLIClearrangeCmd{}, "")
	c.Register(&fdbCLIClearCmd{}, "")
	c.Register(&fdbCLIClearrangeCmd{}, "")
	c.Register(&fdbCLICommitCmd{}, "")
	c.Register(&fdbCLIConfigureCmd{}, "")
	c.Register(&fdbCLIConsistencycheckCmd{}, "")
	c.Register(&fdbCLICoordinatorsCmd{}, "")
	c.Register(&fdbCLICreatetenantCmd{}, "")
	c.Register(&fdbCLIDatadistributionCmd{}, "")
	c.Register(&fdbCLIDefaulttenantCmd{}, "")
	c.Register(&fdbCLIDeletetenantCmd{}, "")
	c.Register(&fdbCLIExcludeCmd{}, "")
	c.Register(&fdbCLIExpensiveDataCheckCmd{}, "")
	c.Register(&fdbCLIFileconfigureCmd{}, "")
	c.Register(&fdbCLIForceRecoveryWithDataLossCmd{}, "")
	c.Register(&fdbCLIGetCmd{}, "")
	c.Register(&fdbCLIGetrangeCmd{}, "")
	c.Register(&fdbCLIGetrangekeysCmd{}, "")
	c.Register(&fdbCLIGettenantCmd{}, "")
	c.Register(&fdbCLIGetversionCmd{}, "")
	c.Register(&fdbCLIHelpCmd{}, "")
	c.Register(&fdbCLIIncludeCmd{}, "")
	c.Register(&fdbCLIKillCmd{}, "")
	c.Register(&fdbCLIListtenantsCmd{}, "")
	c.Register(&fdbCLILockCmd{}, "")
	c.Register(&fdbCLIMaintenanceCmd{}, "")
	c.Register(&fdbCLIOptionCmd{}, "")
	c.Register(&fdbCLIProfileCmd{}, "")
	c.Register(&fdbCLISetCmd{}, "")
	c.Register(&fdbCLISetclassCmd{}, "")
	c.Register(&fdbCLISleepCmd{}, "")
	c.Register(&fdbCLISnapshotCmd{}, "")
	c.Register(&fdbCLIStatusCmd{}, "")
	c.Register(&fdbCLISuspendCmd{}, "")
	c.Register(&fdbCLIThrottleCmd{}, "")
	c.Register(&fdbCLITriggerddteaminfologCmd{}, "")
	c.Register(&fdbCLITssqCmd{}, "")
	c.Register(&fdbCLIUnlockCmd{}, "")
	c.Register(&fdbCLIUsetenantCmd{}, "")
	c.Register(&fdbCLIWritemodeCmd{}, "")
	c.Register(&fdbCLIVersionepochCmd{}, "")
	c.Register(&fdbCLIWaitconnectedCmd{}, "")
	c.Register(&fdbCLIWaitopenCmd{}, "")

	return c
}

type fdbCLICmd struct {
	req  *pb.FDBCLIRequest
	exec string
}

func (*fdbCLICmd) Name() string { return fdbCLIPackage }
func (*fdbCLICmd) Synopsis() string {
	return "Run a fdbcli command on the given host.\n" + client.GenerateSynopsis(setupFDBCLI(flag.NewFlagSet("", flag.ContinueOnError)), 4)
}
func (p *fdbCLICmd) Usage() string {
	return `fdbcli [--cluster-file|-C=X] [--log] [--trace-format=X] [–-tls_certificate_file=X] [–-tls_ca_file=X] [–-tls_key_file=X] [–-tls_password=X] [--tls_verify_peers=X] [--debug-tls] [--version|-v] [--log-group=X] [--no-status] [--memory=X] [--build-flags] [--timeout=N] [--status-from-json] --exec "<command> <options>"

	Run fdbcli for the given command with specified top level flags as well as any command specific flags.

NOTE: unlike native fdbcli no iteractive session is allowed. Either pass --exec and an arg or put a command and options after all flags (--exec is implied in that case)
` + client.GenerateUsage(fdbCLIPackage, p.Synopsis())
}

func (r *fdbCLICmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIRequest{}

	f.Func("exec", "Run the given command and exit", func(s string) error {
		if r.exec != "" {
			return errors.New("can't set exec more than once")
		}
		r.exec = s
		return nil
	})

	// All of these flags are optional so we only want to set them in req if they were actually set on the command line.
	// Use flag.Func to do this. The only negative is we get passed strings for all values so have to do
	// conversions ourselves.

	clusterFileDesc := "The path of a file containing the connection string for the FoundationDB cluster. The default is first the value of the FDB_CLUSTER_FILE environment variable, then ./fdb.cluster, then /etc/foundationdb/fdb.cluster"
	clusterFileFunc := func(s string) error {
		r.req.Config = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	}
	f.Func("cluster-file", clusterFileDesc, clusterFileFunc)
	f.Func("C", clusterFileDesc, clusterFileFunc)

	f.Var(funcValue(func(s string) error {
		r.req.Log = &wrapperspb.BoolValue{
			Value: true,
		}
		return nil
	}), "log", "Enables trace file logging for the CLI session. Implies --log-dir which is handled server side and logs returned in the response.")
	f.Func("trace-format", "Select the format of the log files. xml (the default) and json are supported. Has no effect unless --log is specified.", func(s string) error {
		r.req.TraceFormat = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	})
	f.Func("tls_certificate_file", "The path of a file containing the TLS certificate and CA chain.", func(s string) error {
		r.req.TlsCertificateFile = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	})
	f.Func("tls_ca_file", "The path of a file containing the CA certificates chain.", func(s string) error {
		r.req.TlsCaFile = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	})
	f.Func("tls_key_file", "The path of a file containing the private key corresponding to the TLS certificate.", func(s string) error {
		r.req.TlsKeyFile = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	})
	f.Func("tls_password", "The passphrase of encrypted private key", func(s string) error {
		r.req.TlsPassword = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	})
	f.Func("tls_verify_peers", "The constraints by which to validate TLS peers. The contents and format of CONSTRAINTS are plugin-specific.", func(s string) error {
		r.req.TlsVerifyPeers = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	})
	f.Var(funcValue(func(s string) error {
		r.req.DebugTls = &wrapperspb.BoolValue{
			Value: true,
		}
		return nil
	}), "debug-tls", "Prints the TLS configuration and certificate chain, then exits. Useful in reporting and diagnosing TLS issues.")

	versionDesc := "Print FoundationDB CLI version information and exit."
	versionFunc := func(s string) error {
		r.req.Version = &wrapperspb.BoolValue{
			Value: true,
		}
		return nil
	}
	f.Var(funcValue(versionFunc), "v", versionDesc)
	f.Var(funcValue(versionFunc), "version", versionDesc)

	f.Func("log-group", "Sets the LogGroup field with the specified value for all events in the trace output (defaults to `default').", func(s string) error {
		r.req.LogGroup = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	})
	f.Var(funcValue(func(s string) error {
		r.req.NoStatus = &wrapperspb.BoolValue{
			Value: true,
		}
		return nil
	}), "no-status", "Disables the initial status check done when starting the CLI.")
	f.Func("memory", "Resident memory limit of the CLI (defaults to 8GiB).", func(s string) error {
		r.req.Memory = &wrapperspb.StringValue{
			Value: s,
		}
		return nil
	})
	f.Var(funcValue(func(s string) error {
		r.req.BuildFlags = &wrapperspb.BoolValue{
			Value: true,
		}
		return nil
	}), "build-flags", "Print build information and exit.")
	f.Func("timeout", "timeout for --exec", func(s string) error {
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("can't parse timeout: %v", err)
		}
		r.req.Timeout = &wrapperspb.Int32Value{
			Value: int32(v),
		}
		return nil
	})
}

// Execute acts like the top level fdbCmd does and delegates actual execution to the parsed sub-command.
// It does pass along the top level request since any top level flags need to be filled in there and available
// for the final request. Sub commands will implement the specific oneof Command and any command specific flags.
func (r *fdbCLICmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setupFDBCLI(f)
	// If it's just asking for flags or commands run those and exit. Help is special since we have that
	// as its own subcommand which executes remotely.
	if f.Arg(0) == "flags" || f.Arg(0) == "commands" || f.Arg(0) == "help" {
		return c.Execute(ctx, args...)
	}

	// We allow --exec and also just a trailing command/args to work. The former is to mimic how people are used
	// to doing this from the existing CLI so migration is simpler.
	if f.NArg() > 0 {
		if r.exec != "" {
			fmt.Fprintln(os.Stderr, "Must specify a command after all options or with --exec but not both")
			return subcommands.ExitFailure
		}
	}
	if r.exec != "" {
		// We want this to still use the parser so move the string from --exec in as Args to the flagset
		f = flag.NewFlagSet("", flag.ContinueOnError)
		args := strings.Fields(r.exec)
		f.Parse(args)
	}

	c = setupFDBCLI(f)

	args = append(args, r.req)
	var cmd []string

	// Join everything back into one string and then resplit. So a command of fdbcli "kill; kill X" will
	// parse through here one by one.
	t := strings.Join(f.Args(), " ")
	for _, a := range strings.Split(t, " ") {
		if a == ";" || strings.HasSuffix(a, ";") {
			// Handle standalone ; vs trailing ; on a command item.
			// For standalone ; we can just ignore this entry.
			if len(a) > 1 {
				a = strings.TrimSuffix(a, ";")
				cmd = append(cmd, a)
			}
			f.Parse(cmd)
			exit := c.Execute(ctx, args...)
			if exit != subcommands.ExitSuccess {
				fmt.Fprintln(os.Stderr, "Error parsing command")
				return exit
			}
			cmd = nil
			continue
		}
		cmd = append(cmd, a)
	}
	if len(cmd) != 0 {
		f.Parse(cmd)
		exit := c.Execute(ctx, args...)
		if exit != subcommands.ExitSuccess {
			fmt.Fprintln(os.Stderr, "Error parsing command")
			return exit
		}
	}
	state := args[0].(*util.ExecuteState)

	return r.runFDBCLI(ctx, state)
}

// runFDBCLI does the legwork for final RPC exection since for each command this is the same.
// Send the request and process the response stream. The only difference being the command to name
// in error messages.
func (r *fdbCLICmd) runFDBCLI(ctx context.Context, state *util.ExecuteState) subcommands.ExitStatus {
	c := pb.NewCLIClientProxy(state.Conn)

	stream, err := c.FDBCLIOneMany(ctx, r.req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - fdbcli error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	openFiles := make(map[string]*os.File)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			for _, e := range state.Err {
				fmt.Fprintf(e, "Stream error: %v\n", err)
			}
			retCode = subcommands.ExitFailure
			break
		}
		for _, r := range resp {
			if r.Error != nil {
				fmt.Fprintf(state.Err[r.Index], "fdbcli error: %v\n", r.Error)
				// If any target had errors it needs to be reported for that target but we still
				// need to process responses off the channel. Final return code though should
				// indicate something failed.
				retCode = subcommands.ExitFailure
				continue
			}
			switch t := r.Resp.Response.(type) {
			case *pb.FDBCLIResponse_Output:
				fmt.Fprintf(state.Out[r.Index], "%s", t.Output.Stdout)
				fmt.Fprintf(state.Err[r.Index], "%s", t.Output.Stderr)
				// If it was non-zero we should exit non-zero
				if t.Output.RetCode != 0 {
					retCode = subcommands.ExitFailure
				}
			case *pb.FDBCLIResponse_Log:
				retCode, openFiles = processLog(state, r.Index, t.Log, openFiles)
				if retCode == subcommands.ExitSuccess {
					if _, err := openFiles[t.Log.Filename].Write(t.Log.Contents); err != nil {
						fmt.Fprintf(state.Err[r.Index], "error writing to logfile %s: %v\n", t.Log.Filename, err)
					}
				}
			}
		}
	}
	for _, v := range openFiles {
		v.Close()
	}
	return retCode
}

// anyEmpty will return true if any entry in the passed in slice is an empty string.
func anyEmpty(s []string) bool {
	for _, e := range s {
		if e == "" {
			return true
		}
	}
	return false
}

type fdbCLIAdvanceversionCmd struct {
	req *pb.FDBCLIAdvanceversion
}

func (*fdbCLIAdvanceversionCmd) Name() string { return "advanceversion" }
func (*fdbCLIAdvanceversionCmd) Synopsis() string {
	return "Force the cluster to recover at the specified version"
}
func (*fdbCLIAdvanceversionCmd) Usage() string {
	return "advanceversion <version>"
}

func (r *fdbCLIAdvanceversionCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIAdvanceversion{}
}

func (r *fdbCLIAdvanceversionCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure

	}
	v, err := strconv.ParseInt(f.Arg(0), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't parse version: %v\n", err)
		return subcommands.ExitFailure
	}
	r.req.Version = v

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Advanceversion{
				Advanceversion: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIBeginCmd struct {
	req *pb.FDBCLIBegin
}

func (*fdbCLIBeginCmd) Name() string { return "begin" }
func (*fdbCLIBeginCmd) Synopsis() string {
	return "Begin a new transaction"
}
func (*fdbCLIBeginCmd) Usage() string {
	return "begin"
}

func (r *fdbCLIBeginCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIBegin{}
}

func (r *fdbCLIBeginCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure

	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Begin{
				Begin: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIBlobrangeCmd struct {
	req *pb.FDBCLIBlobrange
}

func (*fdbCLIBlobrangeCmd) Name() string { return "blobrange" }
func (*fdbCLIBlobrangeCmd) Synopsis() string {
	return "blobify (or not) a given key range"
}
func (*fdbCLIBlobrangeCmd) Usage() string {
	return "blobrange <start|stop> <begin key> <end key>"
}

func (r *fdbCLIBlobrangeCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIBlobrange{}
}

func (r *fdbCLIBlobrangeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 3 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	switch f.Arg(0) {
	case "start":
		r.req.Request = &pb.FDBCLIBlobrange_Start{
			Start: &pb.FDBCLIBlobrangeStart{
				BeginKey: f.Arg(1),
				EndKey:   f.Arg(2),
			},
		}
	case "stop":
		r.req.Request = &pb.FDBCLIBlobrange_Stop{
			Stop: &pb.FDBCLIBlobrangeStop{
				BeginKey: f.Arg(1),
				EndKey:   f.Arg(2),
			},
		}
	}
	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Blobrange{
				Blobrange: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLICacheRangeCmd struct {
	req *pb.FDBCLICacheRange
}

func (*fdbCLICacheRangeCmd) Name() string { return "cacherange" }
func (*fdbCLICacheRangeCmd) Synopsis() string {
	return "Mark a key range to add to or remove from storage caches."
}
func (*fdbCLICacheRangeCmd) Usage() string {
	return "cacherange <set|clear> <begin key> <end key>"
}

func (r *fdbCLICacheRangeCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLICacheRange{}
}

func (r *fdbCLICacheRangeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 3 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	switch f.Arg(0) {
	case "set":
		r.req.Request = &pb.FDBCLICacheRange_Set{
			Set: &pb.FDBCLICacheRangeSet{
				BeginKey: f.Arg(1),
				EndKey:   f.Arg(2),
			},
		}
	case "clear":
		r.req.Request = &pb.FDBCLICacheRange_Clear{
			Clear: &pb.FDBCLICacheRangeClear{
				BeginKey: f.Arg(1),
				EndKey:   f.Arg(2),
			},
		}
	}
	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_CacheRange{
				CacheRange: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIChangefeedCmd struct {
	req *pb.FDBCLIChangefeed
}

func (*fdbCLIChangefeedCmd) Name() string { return "changefeed" }
func (*fdbCLIChangefeedCmd) Synopsis() string {
	return "List, setup, destroy or stream changefeeds"
}
func (p *fdbCLIChangefeedCmd) Usage() string {
	return "changefeed <list|register <range_id> <begin> <end>|destroy <range_id>|stop <range_id>|stream <range_id> [start_version [end_version]]|pop <range_id> <version>"
}

func (r *fdbCLIChangefeedCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIChangefeed{}
}

func (r *fdbCLIChangefeedCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() == 0 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	usage := true
	switch f.Arg(0) {
	case "list":
		if f.NArg() != 1 {
			break
		}
		usage = false
		r.req.Request = &pb.FDBCLIChangefeed_List{
			List: &pb.FDBCLIChangefeedList{},
		}
	case "register":
		if f.NArg() != 4 || anyEmpty(f.Args()) {
			break
		}
		usage = false
		r.req.Request = &pb.FDBCLIChangefeed_Register{
			Register: &pb.FDBCLIChangefeedRegister{
				RangeId: f.Arg(1),
				Begin:   f.Arg(2),
				End:     f.Arg(3),
			},
		}
	case "stop":
		if f.NArg() != 2 || anyEmpty(f.Args()) {
			break
		}
		usage = false
		r.req.Request = &pb.FDBCLIChangefeed_Stop{
			Stop: &pb.FDBCLIChangefeedStop{
				RangeId: f.Arg(1),
			},
		}
	case "destroy":
		if f.NArg() != 2 || anyEmpty(f.Args()) {
			break
		}
		usage = false
		r.req.Request = &pb.FDBCLIChangefeed_Destroy{
			Destroy: &pb.FDBCLIChangefeedDestroy{
				RangeId: f.Arg(1),
			},
		}
	case "stream":
		if f.NArg() < 2 || f.NArg() > 4 || anyEmpty(f.Args()) {
			break
		}
		var s, e int64
		var err error
		if f.NArg() > 2 {
			s, err = strconv.ParseInt(f.Arg(2), 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "can't parse start version: %v\n", err)
				break
			}
		}
		if f.NArg() > 3 {
			e, err = strconv.ParseInt(f.Arg(2), 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "can't parse end version: %v\n", err)
				break
			}
		}
		usage = false
		r.req.Request = &pb.FDBCLIChangefeed_Stream{
			Stream: &pb.FDBCLIChangefeedStream{
				RangeId: f.Arg(1),
			},
		}
		switch f.NArg() {
		case 3:
			r.req.GetStream().Type = &pb.FDBCLIChangefeedStream_StartVersion{
				StartVersion: &pb.FDBCLIChangefeedStreamStartVersion{
					StartVersion: s,
				},
			}
		case 4:
			r.req.GetStream().Type = &pb.FDBCLIChangefeedStream_StartEndVersion{
				StartEndVersion: &pb.FDBCLIChangefeedStreamStartEndVersion{
					StartVersion: s,
					EndVersion:   e,
				},
			}
		default:
			r.req.GetStream().Type = &pb.FDBCLIChangefeedStream_NoVersion{
				NoVersion: &pb.FDBCLIChangefeedStreamNoVersion{},
			}
		}
	case "pop":
		if f.NArg() != 3 || anyEmpty(f.Args()) {
			break
		}
		v, err := strconv.ParseInt(f.Arg(1), 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't parse version: %v\n", err)
			break
		}
		usage = false
		r.req.Request = &pb.FDBCLIChangefeed_Pop{
			Pop: &pb.FDBCLIChangefeedStreamPop{
				RangeId: f.Arg(1),
				Version: v,
			},
		}
	}
	if usage {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Changefeed{
				Changefeed: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIClearCmd struct {
	req *pb.FDBCLIClear
}

func (*fdbCLIClearCmd) Name() string { return "clear" }
func (*fdbCLIClearCmd) Synopsis() string {
	return "Clear a key from the database"
}
func (p *fdbCLIClearCmd) Usage() string {
	return "clear <key>"
}

func (r *fdbCLIClearCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIClear{}
}

func (r *fdbCLIClearCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Key = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Clear{
				Clear: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIClearrangeCmd struct {
	req *pb.FDBCLIClearrange
}

func (*fdbCLIClearrangeCmd) Name() string { return "clearrange" }
func (*fdbCLIClearrangeCmd) Synopsis() string {
	return "Clear a range of keys from the database"
}
func (p *fdbCLIClearrangeCmd) Usage() string {
	return "clearrange <begin_key> <end_key>"
}

func (r *fdbCLIClearrangeCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIClearrange{}
}

func (r *fdbCLIClearrangeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 2 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.BeginKey = f.Arg(0)
	r.req.EndKey = f.Arg(1)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Clearrange{
				Clearrange: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLICommitCmd struct {
	req *pb.FDBCLICommit
}

func (*fdbCLICommitCmd) Name() string { return "commit" }
func (*fdbCLICommitCmd) Synopsis() string {
	return "Commit the current transaction"
}
func (p *fdbCLICommitCmd) Usage() string {
	return "commit"
}

func (r *fdbCLICommitCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLICommit{}
}

func (r *fdbCLICommitCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Commit{
				Commit: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIConfigureCmd struct {
	req *pb.FDBCLIConfigure
}

func (*fdbCLIConfigureCmd) Name() string { return "configure" }
func (*fdbCLIConfigureCmd) Synopsis() string {
	return "Change the database configuration"
}
func (p *fdbCLIConfigureCmd) Usage() string {
	return "configure [new|tss] [single|double|triple|three_data_hall|three_datacenter] [ssd|memory] [commit_proxies=<COMMIT_PROXIES>] [grv_proxies=<GRV_PROXIES>] [logs=<LOGS>] [resolvers=<RESOLVERS>] [count=<TSS_COUNT>] [perpetual_storage_wiggle=<WIGGLE_SPEED>] [perpetual_storage_wiggle_locality=<<LOCALITY_KEY>:<LOCALITY_VALUE>|0>] [storage_migration_type={disabled|gradual|aggressive}] [tenant_mode={disabled|optional_experimental|required_experimental]"
}

func (r *fdbCLIConfigureCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIConfigure{}
}

func (r *fdbCLIConfigureCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	// Technically these have an order but can be parsed regardless of position.
	// The only real constraint is each is only set once possibly.
	usage := true
	for _, opt := range f.Args() {
		switch opt {
		case "new", "tss":
			if r.req.NewOrTss != nil {
				fmt.Fprintln(os.Stderr, "new|tss can only be set once")
				break
			}
			usage = false
			r.req.NewOrTss = &wrapperspb.StringValue{
				Value: opt,
			}
		case "single", "double", "triple", "three_data_hall", "three_datacenter":
			if r.req.RedundancyMode != nil {
				fmt.Fprintln(os.Stderr, "redundancy mode can only be set once")
				break
			}
			usage = false
			r.req.RedundancyMode = &wrapperspb.StringValue{
				Value: opt,
			}
		case "ssd", "memory":
			if r.req.StorageEngine != nil {
				fmt.Fprintln(os.Stderr, "storage engine can only be set once")
				break
			}
			usage = false
			r.req.StorageEngine = &wrapperspb.StringValue{
				Value: opt,
			}
		default:
			// Need to be K=V style
			kv := strings.SplitN(opt, "=", 2)
			// foo= will parse as 2 entries just the 2nd is blank.
			if len(kv) != 2 || anyEmpty(kv) {
				fmt.Fprintf(os.Stderr, "can't parse configure option %q\n", opt)
				break
			}
			setUintVal := func(e string, val string, field **wrapperspb.UInt32Value) subcommands.ExitStatus {
				if *field != nil {
					fmt.Fprintf(os.Stderr, "option %s already set\n", opt)
					return subcommands.ExitFailure
				}
				v, err := strconv.ParseInt(val, 10, 32)
				if err != nil {
					fmt.Fprintf(os.Stderr, "can't parse configure option %q\n", e)
					return subcommands.ExitFailure
				}
				*field = &wrapperspb.UInt32Value{
					Value: uint32(v),
				}
				return subcommands.ExitSuccess
			}

			switch kv[0] {
			case "grv_proxies":
				if setUintVal(opt, kv[1], &r.req.GrvProxies) == subcommands.ExitSuccess {
					usage = false
				}
			case "commit_proxies":
				if setUintVal(opt, kv[1], &r.req.CommitProxies) == subcommands.ExitSuccess {
					usage = false
				}
			case "resolvers":
				if setUintVal(opt, kv[1], &r.req.Resolvers) == subcommands.ExitSuccess {
					usage = false
				}
			case "logs":
				if setUintVal(opt, kv[1], &r.req.Logs) == subcommands.ExitSuccess {
					usage = false
				}
			case "count":
				if setUintVal(opt, kv[1], &r.req.Count) == subcommands.ExitSuccess {
					usage = false
				}
			case "perpetual_storage_wiggle":
				if setUintVal(opt, kv[1], &r.req.PerpetualStorageWiggle) == subcommands.ExitSuccess {
					usage = false
				}
			case "perpetual_storage_wiggle_locality":
				if r.req.PerpetualStorageWiggleLocality != nil {
					fmt.Fprintf(os.Stderr, "option %s already set\n", opt)
					break
				}
				usage = false
				r.req.PerpetualStorageWiggleLocality = &wrapperspb.StringValue{
					Value: kv[1],
				}
			case "storage_migration_type":
				if r.req.StorageMigrationType != nil {
					fmt.Fprintf(os.Stderr, "option %s already set\n", opt)
					break
				}
				usage = false
				r.req.StorageMigrationType = &wrapperspb.StringValue{
					Value: kv[1],
				}
			case "tenant_mode":
				if r.req.TenantMode != nil {
					fmt.Fprintf(os.Stderr, "option %s already set\n", opt)
					break
				}
				usage = false
				r.req.TenantMode = &wrapperspb.StringValue{
					Value: kv[1],
				}
			default:
				fmt.Fprintf(os.Stderr, "can't parse configure option %q\n", opt)
			}
		}

	}
	if usage {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Configure{
				Configure: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIConsistencycheckCmd struct {
	req *pb.FDBCLIConsistencycheck
}

func (*fdbCLIConsistencycheckCmd) Name() string { return "consistencycheck" }
func (*fdbCLIConsistencycheckCmd) Synopsis() string {
	return "The consistencycheck command enables or disables consistency checking."
}
func (p *fdbCLIConsistencycheckCmd) Usage() string {
	return "consistencycheck [on|off]"
}

func (r *fdbCLIConsistencycheckCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIConsistencycheck{}
}

func (r *fdbCLIConsistencycheckCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	switch f.Arg(0) {
	case "on":
		r.req.Mode = &wrapperspb.BoolValue{
			Value: true,
		}
	case "off":
		r.req.Mode = &wrapperspb.BoolValue{}
	default:
		fmt.Fprintln(os.Stderr, "Only one option of either on or off can be specified")
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Consistencycheck{
				Consistencycheck: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLICoordinatorsCmd struct {
	req *pb.FDBCLICoordinators
}

func (*fdbCLICoordinatorsCmd) Name() string { return "coordinators" }
func (*fdbCLICoordinatorsCmd) Synopsis() string {
	return "The coordinators command is used to change cluster coordinators or description."
}
func (p *fdbCLICoordinatorsCmd) Usage() string {
	return "coordinators auto|<address...> [description=X]"
}

func (r *fdbCLICoordinatorsCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLICoordinators{}
}

func (r *fdbCLICoordinatorsCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	auto, desc, usage := false, false, false
	for _, opt := range f.Args() {
		if desc {
			fmt.Fprintln(os.Stderr, "description cannot be set more than once and must be last")
			usage = true
			break
		}
		if opt == "auto" {
			if auto {
				fmt.Fprintln(os.Stderr, "auto can only be set once and not after addresses")
				usage = true
				break
			}
			r.req.Request = &pb.FDBCLICoordinators_Auto{
				Auto: &pb.FDBCLICoordinatorsAuto{},
			}
			auto = true
			continue
		}
		if strings.HasPrefix(opt, "description=") {
			if desc {
				fmt.Fprintln(os.Stderr, "description can only be set once")
				usage = true
				break
			}
			d := strings.SplitN(opt, "=", 2)
			if len(d) != 2 || anyEmpty(d) {
				fmt.Fprintln(os.Stderr, "description must be of the form description=X")
				usage = true
				break
			}
			r.req.Description = &wrapperspb.StringValue{
				Value: d[1],
			}
			desc = true
			continue
		}
		// If we get here it must be one of N addresses so auto can't be set now.
		if r.req.Request == nil {
			r.req.Request = &pb.FDBCLICoordinators_Addresses{
				Addresses: &pb.FDBCLICoordinatorsAddresses{},
			}
		}
		r.req.GetAddresses().Addresses = append(r.req.GetAddresses().Addresses, opt)
		auto = true
	}
	if usage {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Coordinators{
				Coordinators: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLICreatetenantCmd struct {
	req *pb.FDBCLICreatetenant
}

func (*fdbCLICreatetenantCmd) Name() string { return "createtenant" }
func (*fdbCLICreatetenantCmd) Synopsis() string {
	return "The createtenant command is used to create new tenants in the cluster"
}
func (p *fdbCLICreatetenantCmd) Usage() string {
	return "createtenant <tenant name>"
}

func (r *fdbCLICreatetenantCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLICreatetenant{}
}

func (r *fdbCLICreatetenantCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Name = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Createtenant{
				Createtenant: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIDatadistributionCmd struct {
	req *pb.FDBCLIDatadistribution
}

func (*fdbCLIDatadistributionCmd) Name() string { return "datadistribution" }
func (*fdbCLIDatadistributionCmd) Synopsis() string {
	return "Set data distribution state/modes"
}
func (p *fdbCLIDatadistributionCmd) Usage() string {
	return "datadistribution <on|off|disable <option>|enable <option>>"
}

func (r *fdbCLIDatadistributionCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIDatadistribution{}
}

func (r *fdbCLIDatadistributionCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() < 1 || f.NArg() > 2 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	switch f.Arg(0) {
	case "on":
		r.req.Request = &pb.FDBCLIDatadistribution_On{
			On: &pb.FDBCLIDatadistributionOn{},
		}
	case "off":
		r.req.Request = &pb.FDBCLIDatadistribution_Off{
			Off: &pb.FDBCLIDatadistributionOff{},
		}
	case "enable":
		r.req.Request = &pb.FDBCLIDatadistribution_Enable{
			Enable: &pb.FDBCLIDatadistributionEnable{
				Option: f.Arg(1),
			},
		}
	case "disable":
		r.req.Request = &pb.FDBCLIDatadistribution_Disable{
			Disable: &pb.FDBCLIDatadistributionDisable{
				Option: f.Arg(1),
			},
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Datadistribution{
				Datadistribution: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIDefaulttenantCmd struct {
	req *pb.FDBCLIDefaulttenant
}

func (*fdbCLIDefaulttenantCmd) Name() string { return "defaulttenant" }
func (*fdbCLIDefaulttenantCmd) Synopsis() string {
	return "The defaulttenant command configures fdbcli to run its commands without a tenant. This is the default behavior."
}
func (p *fdbCLIDefaulttenantCmd) Usage() string {
	return "defaulttenant"
}

func (r *fdbCLIDefaulttenantCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIDefaulttenant{}
}

func (r *fdbCLIDefaulttenantCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Defaulttenant{
				Defaulttenant: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIDeletetenantCmd struct {
	req *pb.FDBCLIDeletetenant
}

func (*fdbCLIDeletetenantCmd) Name() string { return "deletetenant" }
func (*fdbCLIDeletetenantCmd) Synopsis() string {
	return "The deletetenant command is used to delete tenants from the cluster."
}
func (p *fdbCLIDeletetenantCmd) Usage() string {
	return "deletetenant <tenant name>"
}

func (r *fdbCLIDeletetenantCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIDeletetenant{}
}

func (r *fdbCLIDeletetenantCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Name = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Deletetenant{
				Deletetenant: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIExcludeCmd struct {
	req *pb.FDBCLIExclude
}

func (*fdbCLIExcludeCmd) Name() string { return "exclude" }
func (*fdbCLIExcludeCmd) Synopsis() string {
	return "The exclude command excludes servers from the database or marks them as failed."
}
func (p *fdbCLIExcludeCmd) Usage() string {
	return "exclude [failed] [<ADDRESS...>] [locality_dcid:<excludedcid>] [locality_zoneid:<excludezoneid>] [locality_machineid:<excludemachineid>] [locality_processid:<excludeprocessid>] or any locality"
}

func (r *fdbCLIExcludeCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIExclude{}
}

func (r *fdbCLIExcludeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	failed, address := false, false
	for _, opt := range f.Args() {
		if opt == "failed" {
			if failed {
				fmt.Fprintln(os.Stderr, "cannot specify failed more than once")
				fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
				return subcommands.ExitFailure
			}
			if address {
				fmt.Fprintln(os.Stderr, "cannot specify failed after listing addresses")
				fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
				return subcommands.ExitFailure
			}
			r.req.Failed = &wrapperspb.BoolValue{
				Value: true,
			}
			failed = true
			continue
		}
		address = true
		if opt == "" {
			fmt.Fprintln(os.Stderr, "address cannot be blank")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Addresses = append(r.req.Addresses, opt)
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Exclude{
				Exclude: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIExpensiveDataCheckCmd struct {
	req *pb.FDBCLIExpensiveDataCheck
}

func (*fdbCLIExpensiveDataCheckCmd) Name() string { return "expensivedatacheck" }
func (*fdbCLIExpensiveDataCheckCmd) Synopsis() string {
	return "expensivedatacheck is used to send a data check request to the specified process. The check request is accomplished by rebooting the process."
}
func (p *fdbCLIExpensiveDataCheckCmd) Usage() string {
	return "expensivedatacheck [list|all|address...]"
}

func (r *fdbCLIExpensiveDataCheckCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIExpensiveDataCheck{}
}

func (r *fdbCLIExpensiveDataCheckCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	switch f.NArg() {
	case 0:
		r.req.Request = &pb.FDBCLIExpensiveDataCheck_Init{
			Init: &pb.FDBCLIExpensiveDataCheckInit{},
		}
	case 1:
		switch f.Arg(0) {
		case "list":
			r.req.Request = &pb.FDBCLIExpensiveDataCheck_List{
				List: &pb.FDBCLIExpensiveDataCheckList{},
			}
		case "all":
			r.req.Request = &pb.FDBCLIExpensiveDataCheck_All{
				All: &pb.FDBCLIExpensiveDataCheckAll{},
			}
		default:
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
	default:
		r.req.Request = &pb.FDBCLIExpensiveDataCheck_Check{
			Check: &pb.FDBCLIExpensiveDataCheckCheck{},
		}
		r.req.GetCheck().Addresses = append(r.req.GetCheck().Addresses, f.Args()...)
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_ExpensiveDataCheck{
				ExpensiveDataCheck: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIFileconfigureCmd struct {
	req *pb.FDBCLIFileconfigure
}

func (*fdbCLIFileconfigureCmd) Name() string { return "fileconfigure" }
func (*fdbCLIFileconfigureCmd) Synopsis() string {
	return "The fileconfigure command is alternative to the configure command which changes the configuration of the database based on a json document. The command loads a JSON document from the provided file, and change the database configuration to match the contents of the JSON document."
}
func (p *fdbCLIFileconfigureCmd) Usage() string {
	return `fileconfigure [new] <filename>

NOTE: The filename is a path on the remote system which must already exist.
`
}

func (r *fdbCLIFileconfigureCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIFileconfigure{}
}

func (r *fdbCLIFileconfigureCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() < 1 || f.NArg() > 2 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure

	}
	file := f.Arg(0)
	if f.NArg() == 2 {
		if f.Arg(0) != "new" {
			fmt.Fprintln(os.Stderr, "first argument must be \"new\" if 2 args are specified")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.New = &wrapperspb.BoolValue{
			Value: true,
		}
		file = f.Arg(1)
	}

	r.req.File = file

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Fileconfigure{
				Fileconfigure: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIForceRecoveryWithDataLossCmd struct {
	req *pb.FDBCLIForceRecoveryWithDataLoss
}

func (*fdbCLIForceRecoveryWithDataLossCmd) Name() string { return "forcerecoverywithdataloss" }
func (*fdbCLIForceRecoveryWithDataLossCmd) Synopsis() string {
	return "The force_recovery_with_data_loss command will recover a multi-region database to the specified datacenter."
}
func (p *fdbCLIForceRecoveryWithDataLossCmd) Usage() string {
	return "forcerecoverywithdataloss <dcid>"
}

func (r *fdbCLIForceRecoveryWithDataLossCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIForceRecoveryWithDataLoss{}
}

func (r *fdbCLIForceRecoveryWithDataLossCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure

	}
	r.req.Dcid = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_ForceRecoveryWithDataLoss{
				ForceRecoveryWithDataLoss: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIGetCmd struct {
	req *pb.FDBCLIGet
}

func (*fdbCLIGetCmd) Name() string { return "get" }
func (*fdbCLIGetCmd) Synopsis() string {
	return "The get command fetches the value of a given key."
}
func (p *fdbCLIGetCmd) Usage() string {
	return "get <key>"
}

func (r *fdbCLIGetCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIGet{}
}

func (r *fdbCLIGetCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure

	}
	r.req.Key = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Get{
				Get: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIGetrangeCmd struct {
	req *pb.FDBCLIGetrange
}

func (*fdbCLIGetrangeCmd) Name() string { return "getrange" }
func (*fdbCLIGetrangeCmd) Synopsis() string {
	return "The getrange command fetches key-value pairs in a range. "
}
func (p *fdbCLIGetrangeCmd) Usage() string {
	return "getrange <begin key> [end key] [limit]"
}

func (r *fdbCLIGetrangeCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIGetrange{}
}

func (r *fdbCLIGetrangeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() < 1 || f.NArg() > 3 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.BeginKey = f.Arg(0)
	switch f.NArg() {
	case 1:
		// This could be an end key or a limit. If it parses as a integer it's a limit and there
		// can't be anything else. Otherwise it's and end key and the next one must parse as a limit.
		v, err := strconv.ParseInt(f.Arg(1), 10, 32)
		if err != nil {
			r.req.EndKey = &wrapperspb.StringValue{
				Value: f.Arg(1),
			}
			break
		}
		// Otherwise it's a limit if it's a number.
		r.req.Limit = &wrapperspb.UInt32Value{
			Value: uint32(v),
		}
	case 2:
		// 2nd one is a limit so has to be a number.
		v, err := strconv.ParseInt(f.Arg(2), 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't parse limit: %v\n", err)
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.EndKey = &wrapperspb.StringValue{
			Value: f.Arg(1),
		}
		r.req.Limit = &wrapperspb.UInt32Value{
			Value: uint32(v),
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Getrange{
				Getrange: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIGetrangekeysCmd struct {
	req *pb.FDBCLIGetrangekeys
}

func (*fdbCLIGetrangekeysCmd) Name() string { return "getrangekeys" }
func (*fdbCLIGetrangekeysCmd) Synopsis() string {
	return "The getrangekeys command fetches keys in a range. Its syntax is getrangekeys <BEGINKEY> [ENDKEY] [LIMIT]"
}
func (p *fdbCLIGetrangekeysCmd) Usage() string {
	return "getrangekeys <begin key> [end key] [limit]"
}

func (r *fdbCLIGetrangekeysCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIGetrangekeys{}
}

func (r *fdbCLIGetrangekeysCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	// NOTE: This is the same as getkeys but differing proto fields make it annoying to break into a utility function.
	if f.NArg() < 1 || f.NArg() > 3 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.BeginKey = f.Arg(0)
	switch f.NArg() {
	case 1:
		// This could be an end key or a limit. If it parses as a integer it's a limit and there
		// can't be anything else. Otherwise it's and end key and the next one must parse as a limit.
		v, err := strconv.ParseInt(f.Arg(1), 10, 32)
		if err != nil {
			r.req.EndKey = &wrapperspb.StringValue{
				Value: f.Arg(1),
			}
			break
		}
		// Otherwise it's a limit if it's a number.
		r.req.Limit = &wrapperspb.UInt32Value{
			Value: uint32(v),
		}
	case 2:
		// 2nd one is a limit so has to be a number.
		v, err := strconv.ParseInt(f.Arg(2), 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't parse limit: %v\n", err)
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.EndKey = &wrapperspb.StringValue{
			Value: f.Arg(1),
		}
		r.req.Limit = &wrapperspb.UInt32Value{
			Value: uint32(v),
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Getrangekeys{
				Getrangekeys: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIGettenantCmd struct {
	req *pb.FDBCLIGettenant
}

func (*fdbCLIGettenantCmd) Name() string { return "gettenant" }
func (*fdbCLIGettenantCmd) Synopsis() string {
	return "The gettenant command fetches metadata for a given tenant and displays it."
}
func (p *fdbCLIGettenantCmd) Usage() string {
	return "gettenant <tenant name>"
}

func (r *fdbCLIGettenantCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIGettenant{}
}

func (r *fdbCLIGettenantCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Name = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Gettenant{
				Gettenant: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIGetversionCmd struct {
	req *pb.FDBCLIGetversion
}

func (*fdbCLIGetversionCmd) Name() string { return "getversion" }
func (*fdbCLIGetversionCmd) Synopsis() string {
	return "The getversion command fetches the current read version of the cluster or currently running transaction."
}
func (p *fdbCLIGetversionCmd) Usage() string {
	return "getversion"
}

func (r *fdbCLIGetversionCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIGetversion{}
}

func (r *fdbCLIGetversionCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Getversion{
				Getversion: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIHelpCmd struct {
	req *pb.FDBCLIHelp
}

func (*fdbCLIHelpCmd) Name() string { return "fdbclihelp" }
func (*fdbCLIHelpCmd) Synopsis() string {
	return "The help command provides information on specific commands. Its syntax is help <TOPIC>, where <TOPIC> is any of the commands in this section, escaping, or options"
}
func (p *fdbCLIHelpCmd) Usage() string {
	return `fdbclihelp [TOPIC]
NOTE: This actually runs help on the remote fdbcli process but needed to be renamed here due conflicts with standard commands`
}

func (r *fdbCLIHelpCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIHelp{}
}

func (r *fdbCLIHelpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	r.req.Options = append(r.req.Options, f.Args()...)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Help{
				Help: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIIncludeCmd struct {
	req *pb.FDBCLIInclude
}

func (*fdbCLIIncludeCmd) Name() string { return "include" }
func (*fdbCLIIncludeCmd) Synopsis() string {
	return "The include command permits previously excluded or failed servers/localities to rejoin the database."
}
func (p *fdbCLIIncludeCmd) Usage() string {
	return "include [failed] all|[<ADDRESS...>] [locality_dcid:<excludedcid>] [locality_zoneid:<excludezoneid>] [locality_machineid:<excludemachineid>] [locality_processid:<excludeprocessid>] or any locality"
}

func (r *fdbCLIIncludeCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIInclude{}
}

func (r *fdbCLIIncludeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	failed, address, all := false, false, false
	for _, opt := range f.Args() {
		if all {
			fmt.Fprintln(os.Stderr, "can't set any addresses after all is declared")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		switch opt {
		case "failed":
			if failed {
				fmt.Fprintln(os.Stderr, "cannot specify failed more than once")
				fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
				return subcommands.ExitFailure
			}
			if address || all {
				fmt.Fprintln(os.Stderr, "cannot specify failed after listing addresses")
				fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
				return subcommands.ExitFailure
			}
			r.req.Failed = &wrapperspb.BoolValue{
				Value: true,
			}
			failed = true
		case "all":
			if all {
				fmt.Fprintln(os.Stderr, "cannot specify all more than once")
				fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
				return subcommands.ExitFailure
			}
			if address {
				fmt.Fprintln(os.Stderr, "cannot specify all and addresses together")
				fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
				return subcommands.ExitFailure
			}
			r.req.Request = &pb.FDBCLIInclude_All{
				All: true,
			}
			all = true
		default:
			if opt == "" {
				fmt.Fprintln(os.Stderr, "address cannot be blank")
				fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
				return subcommands.ExitFailure
			}
			if r.req.Request == nil {
				r.req.Request = &pb.FDBCLIInclude_Addresses{
					Addresses: &pb.FDBCLIIncludeAddresses{},
				}
			}
			r.req.GetAddresses().Addresses = append(r.req.GetAddresses().Addresses, opt)
			address = true
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Include{
				Include: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIKillCmd struct {
	req *pb.FDBCLIKill
}

func (*fdbCLIKillCmd) Name() string { return "kill" }
func (*fdbCLIKillCmd) Synopsis() string {
	return "The kill command attempts to kill one or more processes in the cluster."
}
func (p *fdbCLIKillCmd) Usage() string {
	return `kill [list|all|address...]

NOTE: Internally this will be converted to kill; kill <address...> to do the required auto fill of addresses needed.
`
}

func (r *fdbCLIKillCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIKill{}
}

func (r *fdbCLIKillCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	switch f.NArg() {
	case 0:
		r.req.Request = &pb.FDBCLIKill_Init{
			Init: &pb.FDBCLIKillInit{},
		}
	case 1:
		switch f.Arg(0) {
		case "list":
			r.req.Request = &pb.FDBCLIKill_List{
				List: &pb.FDBCLIKillList{},
			}
		case "all":
			r.req.Request = &pb.FDBCLIKill_All{
				All: &pb.FDBCLIKillAll{},
			}
		}
	}
	if r.req.Request == nil {
		// If neither of the above matched this is N addresses.
		r.req.Request = &pb.FDBCLIKill_Targets{
			Targets: &pb.FDBCLIKillTargets{},
		}
		r.req.GetTargets().Addresses = append(r.req.GetTargets().Addresses, f.Args()...)

	}
	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Kill{
				Kill: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIListtenantsCmd struct {
	req *pb.FDBCLIListtenants
}

func (*fdbCLIListtenantsCmd) Name() string { return "listtentants" }
func (*fdbCLIListtenantsCmd) Synopsis() string {
	return "The listtenants command prints the names of tenants in the cluster."
}
func (p *fdbCLIListtenantsCmd) Usage() string {
	return "listtenants [<begin> <end>] [limit]"
}

func (r *fdbCLIListtenantsCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIListtenants{}
}

func (r *fdbCLIListtenantsCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() > 3 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	parseLimit := func(arg string) subcommands.ExitStatus {
		v, err := strconv.ParseInt(arg, 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't parse limit: %v\n", err)
			return subcommands.ExitFailure
		}
		r.req.Limit = &wrapperspb.UInt32Value{
			Value: uint32(v),
		}
		return subcommands.ExitSuccess
	}
	switch f.NArg() {
	case 1:
		// Limit only
		if parseLimit(f.Arg(0)) != subcommands.ExitSuccess {
			return subcommands.ExitFailure
		}
	case 2, 3:
		// Begin and end must be first
		r.req.Begin = &wrapperspb.StringValue{
			Value: f.Arg(0),
		}
		r.req.End = &wrapperspb.StringValue{
			Value: f.Arg(1),
		}
		if f.NArg() == 3 {
			if parseLimit(f.Arg(2)) != subcommands.ExitSuccess {
				return subcommands.ExitFailure
			}
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Listtenants{
				Listtenants: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLILockCmd struct {
	req *pb.FDBCLILock
}

func (*fdbCLILockCmd) Name() string { return "lock" }
func (*fdbCLILockCmd) Synopsis() string {
	return "The lock command locks the database with a randomly generated lockUID"
}
func (p *fdbCLILockCmd) Usage() string {
	return "lock"
}

func (r *fdbCLILockCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLILock{}
}

func (r *fdbCLILockCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Lock{
				Lock: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIMaintenanceCmd struct {
	req *pb.FDBCLIMaintenance
}

func (*fdbCLIMaintenanceCmd) Name() string { return "maintenance" }
func (*fdbCLIMaintenanceCmd) Synopsis() string {
	return "The maintenance command marks a particular zone ID (i.e. fault domain) as being under maintenance."
}
func (p *fdbCLIMaintenanceCmd) Usage() string {
	return "maintenance [<on> <zoneid> <seconds>|off]"
}

func (r *fdbCLIMaintenanceCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIMaintenance{}
}

func (r *fdbCLIMaintenanceCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() == 2 || f.NArg() > 3 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure

	}

	switch f.NArg() {
	// Nothing means status only
	case 0:
		r.req.Request = &pb.FDBCLIMaintenance_Status{
			Status: &pb.FDBCLIMaintenanceStatus{},
		}
	// off
	case 1:
		if f.Arg(0) != "off" {
			fmt.Fprintln(os.Stderr, `called with a single argument only - must be "off"`)
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIMaintenance_Off{
			Off: &pb.FDBCLIMaintenanceOff{},
		}
	// on zoneid seconds
	case 3:
		if f.Arg(0) != "on" {
			fmt.Fprintln(os.Stderr, `3 argument case must be "on zoneid seconds"`)
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		v, err := strconv.ParseInt(f.Arg(2), 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't parse seconds: %v\n", err)
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIMaintenance_On{
			On: &pb.FDBCLIMaintenanceOn{
				Zoneid:  f.Arg(1),
				Seconds: uint32(v),
			},
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Maintenance{
				Maintenance: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIOptionCmd struct {
	req *pb.FDBCLIOption
}

func (*fdbCLIOptionCmd) Name() string { return "option" }
func (*fdbCLIOptionCmd) Synopsis() string {
	return "The option command enables or disables an option."
}
func (p *fdbCLIOptionCmd) Usage() string {
	return "option [on <option> [arg]]|[off <option>]"
}

func (r *fdbCLIOptionCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIOption{}
}

func (r *fdbCLIOptionCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() < 2 || f.NArg() > 3 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	// Has to be 2 or 3 at this point based on above checks.
	switch f.Arg(0) {
	case "off", "on":
		// Just to validate.
	default:
		fmt.Fprintln(os.Stderr, "must specify on or off")
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Request = &pb.FDBCLIOption_Arg{
		Arg: &pb.FDBCLIOptionArg{
			State:  f.Arg(0),
			Option: f.Arg(1),
		},
	}
	if f.NArg() == 3 {
		r.req.GetArg().Arg = &wrapperspb.StringValue{
			Value: f.Arg(3),
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Option{
				Option: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIProfileCmd struct {
	req *pb.FDBCLIProfile
}

func (*fdbCLIProfileCmd) Name() string { return "profile" }
func (*fdbCLIProfileCmd) Synopsis() string {
	return "The profile command is used to control various profiling actions."
}
func (p *fdbCLIProfileCmd) Usage() string {
	return `profile client get|<set <rate|"default"> <size|"default">>
profile list
profile flow run <duration> <filename> <process...> (NOTE: filename is ignored and handled server side and returned)
profile heap <process>
`
}

func (r *fdbCLIProfileCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIProfile{}
}

func (r *fdbCLIProfileCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() == 0 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	switch f.Arg(0) {
	case "client":
		errCase := false
		r.req.Request = &pb.FDBCLIProfile_Client{
			Client: &pb.FDBCLIProfileActionClient{},
		}
		switch f.NArg() {
		case 2:
			if f.Arg(1) != "get" {
				errCase = true
				break
			}
			r.req.GetClient().Request = &pb.FDBCLIProfileActionClient_Get{
				Get: &pb.FDBCLIProfileActionClientGet{},
			}
		case 4:
			r.req.GetClient().Request = &pb.FDBCLIProfileActionClient_Set{
				Set: &pb.FDBCLIProfileActionClientSet{},
			}
			if f.Arg(1) != "set" {
				errCase = true
				break
			}
			if f.Arg(2) == "default" {
				r.req.GetClient().GetSet().Rate = &pb.FDBCLIProfileActionClientSet_DefaultRate{}
			} else {
				v, err := strconv.ParseFloat(f.Arg(2), 64)
				if err != nil {
					errCase = true
					break
				}
				r.req.GetClient().GetSet().Rate = &pb.FDBCLIProfileActionClientSet_ValueRate{
					ValueRate: v,
				}
			}
			if f.Arg(3) == "default" {
				r.req.GetClient().GetSet().Size = &pb.FDBCLIProfileActionClientSet_DefaultSize{}
			} else {
				v, err := strconv.ParseInt(f.Arg(2), 10, 64)
				if err != nil {
					errCase = true
					break
				}
				r.req.GetClient().GetSet().Size = &pb.FDBCLIProfileActionClientSet_ValueSize{
					ValueSize: uint64(v),
				}
			}
		default:
			errCase = true
		}
		if errCase {
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
	case "list":
		if f.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "profile list takes no arguments")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIProfile_List{
			List: &pb.FDBCLIProfileActionList{},
		}
	case "flow":
		errCase := false
		switch f.NArg() {
		case 1, 2, 3, 4:
			errCase = true
		case 5:
			if f.Arg(1) != "run" {
				errCase = true
				break
			}
			r.req.Request = &pb.FDBCLIProfile_Flow{
				Flow: &pb.FDBCLIProfileActionFlow{},
			}
			v, err := strconv.ParseInt(f.Arg(2), 10, 64)
			if err != nil {
				errCase = true
				break
			}
			r.req.GetFlow().Duration = uint32(v)
			r.req.GetFlow().Processes = append(r.req.GetFlow().Processes, f.Args()[4:]...)
		}
		if errCase {
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
	case "heap":
		r.req.Request = &pb.FDBCLIProfile_Heap{
			Heap: &pb.FDBCLIProfileActionHeap{
				Process: f.Arg(1),
			},
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Profile{
				Profile: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLISetCmd struct {
	req *pb.FDBCLISet
}

func (*fdbCLISetCmd) Name() string { return "set" }
func (*fdbCLISetCmd) Synopsis() string {
	return "The set command sets a value for a given key."
}
func (p *fdbCLISetCmd) Usage() string {
	return "set <key> <value>"
}

func (r *fdbCLISetCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLISet{}
}

func (r *fdbCLISetCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 2 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure

	}
	r.req.Key = f.Arg(0)
	r.req.Value = f.Arg(1)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Set{
				Set: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLISetclassCmd struct {
	req *pb.FDBCLISetclass
}

func (*fdbCLISetclassCmd) Name() string { return "setclass" }
func (*fdbCLISetclassCmd) Synopsis() string {
	return "The setclass command can be used to change the process class for a given process."
}
func (p *fdbCLISetclassCmd) Usage() string {
	return "setclass [<address> <class>]"
}

func (r *fdbCLISetclassCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLISetclass{}
}

func (r *fdbCLISetclassCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() == 1 || f.NArg() > 2 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	r.req.Request = &pb.FDBCLISetclass_List{
		List: &pb.FDBCLISetclassList{},
	}
	if f.NArg() == 2 {
		r.req.Request = &pb.FDBCLISetclass_Arg{
			Arg: &pb.FDBCLISetclassArg{
				Address: f.Arg(0),
				Class:   f.Arg(1),
			},
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Setclass{
				Setclass: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLISleepCmd struct {
	req *pb.FDBCLISleep
}

func (*fdbCLISleepCmd) Name() string { return "sleep" }
func (*fdbCLISleepCmd) Synopsis() string {
	return "The sleep command inserts a delay before running the next command."
}
func (p *fdbCLISleepCmd) Usage() string {
	return "sleep <seconds>"
}

func (r *fdbCLISleepCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLISleep{}
}

func (r *fdbCLISleepCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	v, err := strconv.ParseInt(f.Arg(0), 10, 32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid seconds: %v\n", err)
		return subcommands.ExitFailure
	}
	r.req.Seconds = uint32(v)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Sleep{
				Sleep: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLISnapshotCmd struct {
	req *pb.FDBCLISnapshot
}

func (*fdbCLISnapshotCmd) Name() string { return "snapshot" }
func (*fdbCLISnapshotCmd) Synopsis() string {
	return "Take a snapshot of the database using the given command."
}
func (p *fdbCLISnapshotCmd) Usage() string {
	return "snapshot <command> [option...]"
}

func (r *fdbCLISnapshotCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLISnapshot{}
}

func (r *fdbCLISnapshotCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() < 1 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Command = f.Arg(0)
	if f.NArg() > 1 {
		r.req.Options = append(r.req.Options, f.Args()[1:]...)
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Snapshot{
				Snapshot: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIStatusCmd struct {
	req *pb.FDBCLIStatus
}

func (*fdbCLIStatusCmd) Name() string { return "status" }
func (*fdbCLIStatusCmd) Synopsis() string {
	return "The status command reports the status of the FoundationDB cluster to which fdbcli is connected."
}
func (p *fdbCLIStatusCmd) Usage() string {
	return "status <style>"
}

func (r *fdbCLIStatusCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIStatus{}
}

func (r *fdbCLIStatusCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() > 1 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	if f.NArg() == 1 {
		r.req.Style = &wrapperspb.StringValue{
			Value: f.Arg(0),
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Status{
				Status: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLISuspendCmd struct {
	req *pb.FDBCLISuspend
}

func (*fdbCLISuspendCmd) Name() string { return "suspend" }
func (*fdbCLISuspendCmd) Synopsis() string {
	return "Suspend the given processes for N seconds. Without arguments will prime the local cache of addresses and prints them out."
}
func (p *fdbCLISuspendCmd) Usage() string {
	return "suspend [<seconds> <address> [address...]]"
}

func (r *fdbCLISuspendCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLISuspend{}
}

func (r *fdbCLISuspendCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	switch {
	case f.NArg() == 0:
		r.req.Request = &pb.FDBCLISuspend_Init{
			Init: &pb.FDBCLISuspendInit{},
		}
	case f.NArg() > 1:
		v, err := strconv.ParseFloat(f.Arg(0), 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't parse seconds as float: %v", err)
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLISuspend_Suspend{
			Suspend: &pb.FDBCLISuspendSuspend{
				Seconds: v,
			},
		}
		r.req.GetSuspend().Addresses = append(r.req.GetSuspend().Addresses, f.Args()[1:]...)
	default:
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Suspend{
				Suspend: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIThrottleCmd struct {
	req *pb.FDBCLIThrottle
}

func (*fdbCLIThrottleCmd) Name() string { return "throttle" }
func (*fdbCLIThrottleCmd) Synopsis() string {
	return "The throttle command is used to inspect and modify the list of throttled transaction tags in the cluster."
}
func (p *fdbCLIThrottleCmd) Usage() string {
	return `throttle on tag <tag> [rate] [duration - N<s|m|h|d>] [priority]
throttle off [all|auto|manual] [tag <tag>] [priority]
throttle enable auto
throttle disable auto
throttle list [<type>] [LIMIT]
`
}

func (r *fdbCLIThrottleCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIThrottle{}
}

func (r *fdbCLIThrottleCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() < 1 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	switch f.Arg(0) {
	case "on":
		if f.Arg(1) != "tag" {
			fmt.Fprintln(os.Stderr, "throttle on is of the form: throttle on tag <tag> [rate] [duration N<s|m|h|d>] [priority]")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIThrottle_On{
			On: &pb.FDBCLIThrottleActionOn{
				Tag: f.Arg(2),
			},
		}
		if f.NArg() > 3 {
			for _, opt := range f.Args()[3:] {
				// See if it parses as a number
				v, err := strconv.ParseInt(opt, 10, 32)
				if err == nil {
					if r.req.GetOn().Rate != nil {
						fmt.Fprintln(os.Stderr, "can't set rate more than once for throttle on")
						fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
						return subcommands.ExitFailure
					}
					r.req.GetOn().Rate = &wrapperspb.UInt32Value{
						Value: uint32(v),
					}
					continue
				}
				// If it's a number except for the last character it's a duration.
				_, err = strconv.ParseInt(opt[0:len(opt)-2], 10, 32)
				if err == nil {
					if r.req.GetOn().Duration != nil {
						fmt.Fprintln(os.Stderr, "can't set duration more than once for throttle on")
						fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
						return subcommands.ExitFailure
					}
					r.req.GetOn().Duration = &wrapperspb.StringValue{
						Value: opt,
					}
				}
				// Everything else is a priority
				if r.req.GetOn().Priority != nil {
					fmt.Fprintln(os.Stderr, "can't set priority more than once for throttle on")
					fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
					return subcommands.ExitFailure
				}
				r.req.GetOn().Priority = &wrapperspb.StringValue{
					Value: opt,
				}
			}
		}
	case "off":
		r.req.Request = &pb.FDBCLIThrottle_Off{
			Off: &pb.FDBCLIThrottleActionOff{},
		}
		for i := 1; i < f.NArg(); i++ {
			switch f.Arg(i) {
			case "all", "auto", "manual":
				if r.req.GetOff().Type != nil {
					fmt.Fprintln(os.Stderr, "throttle off can only specify one type")
					fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
					return subcommands.ExitFailure
				}
				r.req.GetOff().Type = &wrapperspb.StringValue{
					Value: f.Arg(i),
				}
			case "tag":
				// Advance to look at the tag value
				i++
				if i >= f.NArg() {
					fmt.Fprintln(os.Stderr, "throttle off tag must include a tag value")
					fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
					return subcommands.ExitFailure
				}
				if r.req.GetOff().Tag != nil {
					fmt.Fprintln(os.Stderr, "throttle off can only specify one tag")
					fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
					return subcommands.ExitFailure
				}
				r.req.GetOff().Tag = &wrapperspb.StringValue{
					Value: f.Arg(i),
				}
			default:
				// Anything else is a priority
				if r.req.GetOff().Priority != nil {
					fmt.Fprintln(os.Stderr, "throttle off can only specify one priority")
					fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
					return subcommands.ExitFailure
				}
				r.req.GetOff().Priority = &wrapperspb.StringValue{
					Value: f.Arg(i),
				}
			}
		}
	case "enable":
		if f.NArg() != 2 || f.Arg(1) != "auto" {
			fmt.Fprintln(os.Stderr, "throttle enable is of the form: throttle enable auto")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIThrottle_Enable{
			Enable: &pb.FDBCLIThrottleActionEnable{},
		}
	case "disable":
		if f.NArg() != 2 || f.Arg(1) != "auto" {
			fmt.Fprintln(os.Stderr, "throttle disable is of the form: throttle disable auto")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIThrottle_Disable{
			Disable: &pb.FDBCLIThrottleActionDisable{},
		}
	case "list":
		r.req.Request = &pb.FDBCLIThrottle_List{
			List: &pb.FDBCLIThrottleActionList{},
		}
		errCase := false
		switch f.NArg() {
		case 1:
			// Nothing to do
		case 2:
			// This can be a type or a limit so see if it parses as a number.
			v, err := strconv.ParseInt(f.Arg(1), 10, 32)
			if err != nil {
				r.req.GetList().Type = &wrapperspb.StringValue{
					Value: f.Arg(1),
				}
			} else {
				r.req.GetList().Limit = &wrapperspb.UInt32Value{
					Value: uint32(v),
				}
			}
		case 3:
			// This must be a type and a limit.
			r.req.GetList().Type = &wrapperspb.StringValue{
				Value: f.Arg(1),
			}
			v, err := strconv.ParseInt(f.Arg(2), 10, 32)
			if err != nil {
				errCase = true
				break
			}
			r.req.GetList().Limit = &wrapperspb.UInt32Value{
				Value: uint32(v),
			}
		default:
			errCase = true
		}
		if errCase {
			fmt.Fprintln(os.Stderr, "throttle list is of the form: throttle list [throttled|recommended|all] [LIMIT]")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Throttle{
				Throttle: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLITriggerddteaminfologCmd struct {
	req *pb.FDBCLITriggerddteaminfolog
}

func (*fdbCLITriggerddteaminfologCmd) Name() string { return "triggerddteaminfolog" }
func (*fdbCLITriggerddteaminfologCmd) Synopsis() string {
	return "The triggerddteaminfolog command would trigger the data distributor to log very detailed teams information into trace event logs."
}
func (p *fdbCLITriggerddteaminfologCmd) Usage() string {
	return "triggerddteaminfolog"
}

func (r *fdbCLITriggerddteaminfologCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLITriggerddteaminfolog{}
}

func (r *fdbCLITriggerddteaminfologCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure

	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Triggerddteaminfolog{
				Triggerddteaminfolog: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLITssqCmd struct {
	req *pb.FDBCLITssq
}

func (*fdbCLITssqCmd) Name() string { return "tssq" }
func (*fdbCLITssqCmd) Synopsis() string {
	return "Utility commands for handling quarantining Testing Storage Servers."
}
func (p *fdbCLITssqCmd) Usage() string {
	return `tssq start <StorageUID>
tssq stop <StorageUID>
tssq list
`
}

func (r *fdbCLITssqCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLITssq{}
}

func (r *fdbCLITssqCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() < 1 || f.NArg() > 2 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	switch f.Arg(0) {
	case "start":
		if f.NArg() != 2 {
			fmt.Fprintln(os.Stderr, "tssq start required a storage uid")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLITssq_Start{
			Start: &pb.FDBCLITssqStart{
				StorageUid: f.Arg(1),
			},
		}
	case "stop":
		if f.NArg() != 2 {
			fmt.Fprintln(os.Stderr, "tssq stop required a storage uid")
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLITssq_Stop{
			Stop: &pb.FDBCLITssqStop{
				StorageUid: f.Arg(1),
			},
		}
	case "list":
		r.req.Request = &pb.FDBCLITssq_List{
			List: &pb.FDBCLITssqList{},
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Tssq{
				Tssq: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIUnlockCmd struct {
	req *pb.FDBCLIUnlock
}

func (*fdbCLIUnlockCmd) Name() string { return "unlock" }
func (*fdbCLIUnlockCmd) Synopsis() string {
	return "The unlock command unlocks the database with the specified lock UID"
}
func (p *fdbCLIUnlockCmd) Usage() string {
	return "unlock <uid>"
}

func (r *fdbCLIUnlockCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIUnlock{}
}

func (r *fdbCLIUnlockCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Uid = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Unlock{
				Unlock: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIUsetenantCmd struct {
	req *pb.FDBCLIUsetenant
}

func (*fdbCLIUsetenantCmd) Name() string { return "usetenant" }
func (*fdbCLIUsetenantCmd) Synopsis() string {
	return "The usetenant command configures fdbcli to run transactions within the specified tenant."
}
func (p *fdbCLIUsetenantCmd) Usage() string {
	return "usetenant <name>"
}

func (r *fdbCLIUsetenantCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIUsetenant{}
}

func (r *fdbCLIUsetenantCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Name = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Usetenant{
				Usetenant: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIWritemodeCmd struct {
	req *pb.FDBCLIWritemode
}

func (*fdbCLIWritemodeCmd) Name() string { return "writemode" }
func (*fdbCLIWritemodeCmd) Synopsis() string {
	return "Controls whether or not fdbcli can perform sets and clears."
}
func (p *fdbCLIWritemodeCmd) Usage() string {
	return "writemode <mode>"
}

func (r *fdbCLIWritemodeCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIWritemode{}
}

func (r *fdbCLIWritemodeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 1 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	r.req.Mode = f.Arg(0)

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Writemode{
				Writemode: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIVersionepochCmd struct {
	req *pb.FDBCLIVersionepoch
}

func (*fdbCLIVersionepochCmd) Name() string { return "versionepoch" }
func (*fdbCLIVersionepochCmd) Synopsis() string {
	return "Query/enable/disable and set the version epoch"
}
func (p *fdbCLIVersionepochCmd) Usage() string {
	return "versionepoch [get|disable|enable|commit|set <epoch>]"
}

func (r *fdbCLIVersionepochCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIVersionepoch{}
}

func (r *fdbCLIVersionepochCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() > 2 || anyEmpty(f.Args()) {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}
	switch f.NArg() {
	case 0:
		r.req.Request = &pb.FDBCLIVersionepoch_Info{
			Info: &pb.FDBCLIVersionepochInfo{},
		}
	case 1:
		switch f.Arg(0) {
		case "get":
			r.req.Request = &pb.FDBCLIVersionepoch_Get{
				Get: &pb.FDBCLIVersionepochGet{},
			}
		case "disable":
			r.req.Request = &pb.FDBCLIVersionepoch_Disable{
				Disable: &pb.FDBCLIVersionepochDisable{},
			}
		case "enable":
			r.req.Request = &pb.FDBCLIVersionepoch_Enable{
				Enable: &pb.FDBCLIVersionepochEnable{},
			}
		case "commit":
			r.req.Request = &pb.FDBCLIVersionepoch_Commit{
				Commit: &pb.FDBCLIVersionepochCommit{},
			}
		default:
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
	case 2:
		v, err := strconv.ParseInt(f.Arg(1), 10, 64)
		if f.Arg(0) != "set" || err != nil {
			if err != nil {
				fmt.Fprintf(os.Stderr, "can't parse epoch: %v\n", err)
			}
			fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIVersionepoch_Set{
			Set: &pb.FDBCLIVersionepochSet{
				Epoch: v,
			},
		}
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Versionepoch{
				Versionepoch: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIWaitconnectedCmd struct {
	req *pb.FDBCLIWaitconnected
}

func (*fdbCLIWaitconnectedCmd) Name() string { return "waitconnected" }
func (*fdbCLIWaitconnectedCmd) Synopsis() string {
	return "Pause and wait until connected to the DB"
}
func (p *fdbCLIWaitconnectedCmd) Usage() string {
	return "waitconnected"
}

func (r *fdbCLIWaitconnectedCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIWaitconnected{}
}

func (r *fdbCLIWaitconnectedCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Waitconnected{
				Waitconnected: r.req,
			},
		})

	return subcommands.ExitSuccess
}

type fdbCLIWaitopenCmd struct {
	req *pb.FDBCLIWaitopen
}

func (*fdbCLIWaitopenCmd) Name() string { return "waitopen" }
func (*fdbCLIWaitopenCmd) Synopsis() string {
	return "Pause and wait until a connection to the DB is opened."
}
func (p *fdbCLIWaitopenCmd) Usage() string {
	return "waitopen"
}

func (r *fdbCLIWaitopenCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIWaitopen{}
}

func (r *fdbCLIWaitopenCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	req := args[1].(*pb.FDBCLIRequest)

	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: ", r.Usage())
		return subcommands.ExitFailure
	}

	req.Commands = append(req.Commands,
		&pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Waitopen{
				Waitopen: r.req,
			},
		})

	return subcommands.ExitSuccess
}

const fdbConfCLIPackage = "conf"

func setupConfCLI(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(fdbConfCLIPackage, f)
	c.Register(&fdbConfReadCmd{}, "")
	c.Register(&fdbConfWriteCmd{}, "")
	c.Register(&fdbConfDeleteCmd{}, "")

	return c
}

type fdbConfCmd struct{}

func (*fdbConfCmd) Name() string             { return fdbConfCLIPackage }
func (*fdbConfCmd) SetFlags(_ *flag.FlagSet) {}
func (*fdbConfCmd) Synopsis() string {
	return "Read or update values in foundationdb conf file.\n" + client.GenerateSynopsis(setupConfCLI(flag.NewFlagSet("", flag.ContinueOnError)), 4)
}
func (p *fdbConfCmd) Usage() string {
	return client.GenerateUsage(fdbConfCLIPackage, p.Synopsis())
}

func (p *fdbConfCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setupConfCLI(f)
	return c.Execute(ctx, args...)
}

type fdbConfReadCmd struct {
	path string
}

func (*fdbConfReadCmd) Name() string { return "read" }
func (*fdbConfReadCmd) Synopsis() string {
	return "Read value from a section for a given key and return a response."
}
func (*fdbConfReadCmd) Usage() string {
	return `read <section> <key> [--path <config>]:
Read a key from a section specified in a FDB config file.

Default location for config file is /etc/foundationdb/foundationdb.conf.
`
}

func (r *fdbConfReadCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&r.path, "path", "/etc/foundationdb/foundationdb.conf", "The absolute path to FDB config.")
}

func (r *fdbConfReadCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewConfClientProxy(state.Conn)

	if len(f.Args()) != 2 {
		fmt.Fprint(os.Stderr, "invalid number of parameters: specify section and key only")
		return subcommands.ExitFailure
	}

	section := f.Args()[0]
	key := f.Args()[1]

	resp, err := c.ReadOneMany(ctx, &pb.ReadRequest{Location: &pb.Location{Section: section, Key: key, File: r.path}})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "fdb config read error: %v\n", err)
		}

		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "fdb config read error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}

type fdbConfWriteCmd struct {
	path string
}

func (*fdbConfWriteCmd) Name() string { return "write" }
func (*fdbConfWriteCmd) Synopsis() string {
	return "Write a key-value pair to a section of an FDB config."
}

func (*fdbConfWriteCmd) Usage() string {
	return `write <section> <key> <value> [--path <config>]:
Write a key-value pair to a section specified in a FDB config file.

Default location for config file is /etc/foundationdb/foundationdb.conf.
`
}

func (w *fdbConfWriteCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&w.path, "path", "/etc/foundationdb/foundationdb.conf", "The absolute path to FDB config.")
}

func (w *fdbConfWriteCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewConfClientProxy(state.Conn)

	if len(f.Args()) != 3 {
		fmt.Fprint(os.Stderr, "invalid number of parameters: specify section, key and value only")
		return subcommands.ExitFailure
	}

	section, key, value := f.Args()[0], f.Args()[1], f.Args()[2]

	resp, err := c.WriteOneMany(ctx, &pb.WriteRequest{
		Location: &pb.Location{Section: section, Key: key, File: w.path},
		Value:    value,
	})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "fdb config write error: %v\n", err)
		}

		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "fdb config write error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}

type fdbConfDeleteCmd struct {
	path          string
	deleteSection bool
}

func (*fdbConfDeleteCmd) Name() string { return "delete" }
func (*fdbConfDeleteCmd) Synopsis() string {
	return "Delete key from a section or entire section"
}

func (*fdbConfDeleteCmd) Usage() string {
	return `delete [--delete-section] <section> [<key>]:
Delete key from a section or entire section.
When entire section is deleted, '--delete-section' flag is mandatory but 'key' is optional.

Default location for config file is /etc/foundationdb/foundationdb.conf.
`
}

func (d *fdbConfDeleteCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&d.path, "path", "/etc/foundationdb/foundationdb.conf", "The absolute path to FDB config.")
	f.BoolVar(&d.deleteSection, "delete-section", false, "Delete section safeguard.")
}

func (d *fdbConfDeleteCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewConfClientProxy(state.Conn)

	if len(f.Args()) < 1 {
		fmt.Fprint(os.Stderr, "invalid number of parameters: specify section name.")
		return subcommands.ExitFailure
	}

	if len(f.Args()) > 2 {
		fmt.Fprint(os.Stderr, "invalid number of parameters: specify section name and optional key only.")
		return subcommands.ExitFailure
	}

	section := f.Args()[0]

	var key string
	if len(f.Args()) == 2 {
		key = f.Args()[1]
	}

	if key == "" && !d.deleteSection {
		fmt.Fprint(os.Stderr, "invalid parameters: `delete-section` must be set if `key` is empty.")
		return subcommands.ExitFailure
	}

	if key != "" && d.deleteSection {
		fmt.Fprint(os.Stderr, "invalid parameters: `delete-section` and `key` are mutually exclusive.")
		return subcommands.ExitFailure
	}

	resp, err := c.DeleteOneMany(ctx, &pb.DeleteRequest{Location: &pb.Location{Section: section, Key: key, File: d.path}})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "fdb config delete error: %v\n", err)
		}

		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "fdb config delete error: %v\n", r.Error)
			retCode = subcommands.ExitFailure
		}
	}

	return retCode
}
