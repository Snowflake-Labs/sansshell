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

func processLogs(state *util.ExecuteState, index int, logs []*pb.Log) subcommands.ExitStatus {
	files := make(map[string]bool)
	retCode := subcommands.ExitSuccess
	for _, l := range logs {
		fn := filepath.Base(l.Filename)
		// If the same basename exists we'll try and
		// create a new one with -NUMBER on the end.
		// Start at one until we find a unique one.
		if files[fn] {
			i := 1
			for {
				fn = fmt.Sprintf("%s-%d", fn, i)
				if !files[fn] {
					break
				}
				i++
			}
		}
		files[fn] = true
		if fn == "" {
			fmt.Fprintf(state.Err[index], "RPC returned an invalid log structure: %+v", l)
			retCode = subcommands.ExitFailure
			continue
		}
		// Now create index based names for each log.
		fn = fmt.Sprintf("%d.%s", index, fn)
		path := filepath.Join(state.Dir, fn)
		err := os.WriteFile(path, l.Contents, 0644)
		if err != nil {
			fmt.Fprintf(state.Err[index], "can't write logfile %s: %v", path, err)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&fdbCLICmd{}, "")

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
	c.Register(&fdbCLIClearCmd{}, "")
	c.Register(&fdbCLIClearrangeCmd{}, "")
	c.Register(&fdbCLIConfigureCmd{}, "")
	c.Register(&fdbCLIConsistencycheckCmd{}, "")
	c.Register(&fdbCLICoordinatorsCmd{}, "")
	c.Register(&fdbCLICreatetenantCmd{}, "")
	c.Register(&fdbCLIDefaulttenantCmd{}, "")
	c.Register(&fdbCLIDeletetenantCmd{}, "")
	c.Register(&fdbCLIExcludeCmd{}, "")
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
	c.Register(&fdbCLIStatusCmd{}, "")
	c.Register(&fdbCLIThrottleCmd{}, "")
	c.Register(&fdbCLITriggerddteaminfologCmd{}, "")
	c.Register(&fdbCLIUnlockCmd{}, "")
	c.Register(&fdbCLIUsetenantCmd{}, "")
	c.Register(&fdbCLIWritemodeCmd{}, "")
	c.Register(&fdbCLITssqCmd{}, "")

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

// Execute acts like the top level metadataCmd does and delegates actual execution to the parsed sub-command.
// It does pass along the top level request since any top level flags need to be filled in there and available
// for the final request. Sub commands will implement the specific oneof Command and any command specific flags.
func (r *fdbCLICmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	// We allow --exec and also just a trailing command/args to work. The former is to mimic how people are used
	// to doing this from the existing CLI so migration is simpler.
	if f.NArg() < 1 {
		if r.exec == "" {
			fmt.Fprintln(os.Stderr, "Must specify a command after all options or with --exec")
			return subcommands.ExitFailure
		}
		// We want this to still use the parser so move the string from --exec in as Args to the flagset
		f = flag.NewFlagSet("", flag.ContinueOnError)
		args := strings.Fields(r.exec)
		f.Parse(args)
	}
	c := setupFDBCLI(f)
	args = append(args, r.req)
	return c.Execute(ctx, args...)
}

// runFDBCLI does the legwork for final RPC exection since for each command this is the same.
// Send the request and process the response stream. The only difference being the command to name
// in error messages.
func runFDBCLI(ctx context.Context, c pb.CLIClientProxy, state *util.ExecuteState, req *pb.FDBCLIRequest, command string) subcommands.ExitStatus {
	resp, err := c.FDBCLIOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "fdbcli %s error: %v\n", command, err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "fdbcli %s error: %v\n", command, r.Error)
			// If any target had errors it needs to be reported for that target but we still
			// need to process responses off the channel. Final return code though should
			// indicate something failed.
			retCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprintf(state.Out[r.Index], "%s", r.Resp.Stdout)
		fmt.Fprintf(state.Err[r.Index], "%s", r.Resp.Stderr)
		// If it was non-zero we should exit non-zero
		if r.Resp.RetCode != 0 {
			retCode = subcommands.ExitFailure
		}
		r := processLogs(state, r.Index, r.Resp.Logs)
		if r != subcommands.ExitSuccess {
			retCode = r
		}
	}
	return retCode
}

type fdbCLIAdvanceversionCmd struct {
	req *pb.FDBCLIAdvanceversion
}

func (*fdbCLIAdvanceversionCmd) Name() string { return "advanceversion" }
func (*fdbCLIAdvanceversionCmd) Synopsis() string {
	return "Force the cluster to recover at the specified version"
}
func (p *fdbCLIAdvanceversionCmd) Usage() string {
	return "advanceversion <version>"
}

func (r *fdbCLIAdvanceversionCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIAdvanceversion{}
}

func (r *fdbCLIAdvanceversionCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "must specify one version")
		return subcommands.ExitFailure

	}
	v, err := strconv.ParseInt(f.Arg(0), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't parse version: %v\n", err)
		return subcommands.ExitFailure
	}
	r.req.Version = v

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Advanceversion{
				Advanceversion: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "advanceversion")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "must specify a key")
		return subcommands.ExitFailure

	}
	r.req.Key = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Clear{
				Clear: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "clear")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 2 || f.Arg(0) == "" || f.Arg(1) == "" {
		fmt.Fprintln(os.Stderr, "must specify a begin key and an end key")
		return subcommands.ExitFailure

	}
	r.req.BeginKey = f.Arg(0)
	r.req.EndKey = f.Arg(1)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Clearrange{
				Clearrange: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "clearrange")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Must supply at least one configure option")
		return subcommands.ExitFailure
	}

	// Technically these have an order but can be parsed regardless of position.
	// The only real constraint is each is only set once possibly.
	for _, opt := range f.Args() {
		switch opt {
		case "new", "tss":
			if r.req.NewOrTss != nil {
				fmt.Fprintln(os.Stderr, "new|tss can only be set once")
			}
			r.req.NewOrTss = &wrapperspb.StringValue{
				Value: opt,
			}
		case "single", "double", "triple", "three_data_hall", "three_datacenter":
			if r.req.RedundancyMode != nil {
				fmt.Fprintln(os.Stderr, "redundancy mode can only be set once")
			}
			r.req.RedundancyMode = &wrapperspb.StringValue{
				Value: opt,
			}
		case "ssd", "memory":
			if r.req.StorageEngine != nil {
				fmt.Fprintln(os.Stderr, "storage engine can only be set once")
			}
			r.req.StorageEngine = &wrapperspb.StringValue{
				Value: opt,
			}
		default:
			// Need to be K=V style
			kv := strings.SplitN(opt, "=", 2)
			// foo= will parse as 2 entries just the 2nd is blank.
			if len(kv) != 2 || (len(kv) == 2 && kv[1] == "") {
				fmt.Fprintf(os.Stderr, "can't parse configure option %q\n", opt)
				return subcommands.ExitFailure
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
				if setUintVal(opt, kv[1], &r.req.GrvProxies) != subcommands.ExitSuccess {
					return subcommands.ExitFailure
				}
			case "commit_proxies":
				if setUintVal(opt, kv[1], &r.req.CommitProxies) != subcommands.ExitSuccess {
					return subcommands.ExitFailure
				}
			case "resolvers":
				if setUintVal(opt, kv[1], &r.req.Resolvers) != subcommands.ExitSuccess {
					return subcommands.ExitFailure
				}
			case "logs":
				if setUintVal(opt, kv[1], &r.req.Logs) != subcommands.ExitSuccess {
					return subcommands.ExitFailure
				}
			case "count":
				if setUintVal(opt, kv[1], &r.req.Count) != subcommands.ExitSuccess {
					return subcommands.ExitFailure
				}
			case "perpetual_storage_wiggle":
				if setUintVal(opt, kv[1], &r.req.PerpetualStorageWiggle) != subcommands.ExitSuccess {
					return subcommands.ExitFailure
				}
			case "perpetual_storage_wiggle_locality":
				if r.req.PerpetualStorageWiggleLocality != nil {
					fmt.Fprintf(os.Stderr, "option %s already set\n", opt)
					return subcommands.ExitFailure
				}
				r.req.PerpetualStorageWiggleLocality = &wrapperspb.StringValue{
					Value: kv[1],
				}
			case "storage_migration_type":
				if r.req.StorageMigrationType != nil {
					fmt.Fprintf(os.Stderr, "option %s already set\n", opt)
					return subcommands.ExitFailure
				}
				r.req.StorageMigrationType = &wrapperspb.StringValue{
					Value: kv[1],
				}
			case "tenant_mode":
				if r.req.TenantMode != nil {
					fmt.Fprintf(os.Stderr, "option %s already set\n", opt)
					return subcommands.ExitFailure
				}
				r.req.TenantMode = &wrapperspb.StringValue{
					Value: kv[1],
				}
			default:
				fmt.Fprintf(os.Stderr, "can't parse configure option %q\n", opt)
				return subcommands.ExitFailure
			}
		}
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Configure{
				Configure: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "configure")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "Only one option of either on or off can be specified")
		return subcommands.ExitFailure
	}
	if f.NArg() == 1 {
		switch f.Args()[0] {
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

	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Consistencycheck{
				Consistencycheck: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "consistencycheck")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "must specify auto or at least one address")
		return subcommands.ExitFailure
	}

	auto, desc := false, false
	for _, opt := range f.Args() {
		if desc {
			fmt.Fprintln(os.Stderr, "description cannot be set more than once and must be last")
			return subcommands.ExitFailure
		}
		if opt == "auto" {
			if auto {
				fmt.Fprintln(os.Stderr, "auto can only be set once and not after addresses")
				return subcommands.ExitFailure
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
				return subcommands.ExitFailure
			}
			d := strings.SplitN(opt, "=", 2)
			if len(d) != 2 || (len(d) == 2 && d[1] == "") {
				fmt.Fprintln(os.Stderr, "description must be of the form description=X")
				return subcommands.ExitFailure
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

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Coordinators{
				Coordinators: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "coordinators")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "must specify a tenant name")
		return subcommands.ExitFailure
	}
	r.req.Name = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Createtenant{
				Createtenant: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "createtenant")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "no additional arguments accepted")
		return subcommands.ExitFailure
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Defaulttenant{
				Defaulttenant: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "defaulttenant")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "must specify a tenant name")
		return subcommands.ExitFailure
	}
	r.req.Name = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Deletetenant{
				Deletetenant: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "deletetenant")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	failed, address := false, false
	for _, opt := range f.Args() {
		if opt == "failed" {
			if failed {
				fmt.Fprintln(os.Stderr, "cannot specify failed more than once")
				return subcommands.ExitFailure
			}
			if address {
				fmt.Fprintln(os.Stderr, "cannot specify failed after listing addresses")
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
			return subcommands.ExitFailure
		}
		r.req.Addresses = append(r.req.Addresses, opt)
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Exclude{
				Exclude: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "exclude")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() > 2 || f.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "must specify optionally \"new\" and a filename")
		return subcommands.ExitFailure

	}
	file := f.Arg(0)
	if f.NArg() == 2 {
		if f.Arg(0) != "new" {
			fmt.Fprintln(os.Stderr, "first argument must be \"new\" if 2 args are specified")
			return subcommands.ExitFailure
		}
		r.req.New = &wrapperspb.BoolValue{
			Value: true,
		}
		file = f.Arg(1)
	}

	if file == "" {
		fmt.Fprintln(os.Stderr, "filename cannot be blank")
		return subcommands.ExitFailure
	}
	r.req.File = file

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Fileconfigure{
				Fileconfigure: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "fileconfigure")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "must specify dcid")
		return subcommands.ExitFailure

	}
	r.req.Dcid = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_ForceRecoveryWithDataLoss{
				ForceRecoveryWithDataLoss: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "forcerecoverywithdataloss")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "must specify key")
		return subcommands.ExitFailure

	}
	r.req.Key = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Get{
				Get: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "get")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() < 1 || f.NArg() > 3 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "must specify at least a begin key and optionally end key and/or limit")
		return subcommands.ExitFailure
	}
	r.req.BeginKey = f.Arg(0)
	if f.NArg() > 1 {
		// This could be an end key or a limit. If it parses as a integer it's a limit and there
		// can't be anything else. Otherwise it's and end key and the next one must parse as a limit.
		for _, opt := range f.Args()[1:] {
			if opt == "" {
				fmt.Fprintln(os.Stderr, "end key/limit cannot be blank if set")
				return subcommands.ExitFailure
			}
			if r.req.Limit != nil {
				fmt.Fprintln(os.Stderr, "limit already set, nothing else can be set")
				return subcommands.ExitFailure
			}
			v, err := strconv.ParseInt(f.Arg(1), 10, 32)
			if err != nil {
				if r.req.EndKey != nil {
					fmt.Fprintln(os.Stderr, "end key already set")
					return subcommands.ExitFailure
				}
				r.req.EndKey = &wrapperspb.StringValue{
					Value: f.Arg(1),
				}
				continue
			}
			r.req.Limit = &wrapperspb.UInt32Value{
				Value: uint32(v),
			}
		}
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Getrange{
				Getrange: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "getrange")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	// NOTE: This is the same as getkeys but differing proto fields make it annoying to break into a utility function.
	if f.NArg() < 1 || f.NArg() > 3 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "must specify at least a begin key and optionally end key and/or limit")
		return subcommands.ExitFailure
	}
	r.req.BeginKey = f.Arg(0)
	if f.NArg() > 1 {
		// This could be an end key or a limit. If it parses as a integer it's a limit and there
		// can't be anything else. Otherwise it's and end key and the next one must parse as a limit.
		for _, opt := range f.Args()[1:] {
			if opt == "" {
				fmt.Fprintln(os.Stderr, "end key/limit cannot be blank if set")
				return subcommands.ExitFailure
			}
			if r.req.Limit != nil {
				fmt.Fprintln(os.Stderr, "limit already set, nothing else can be set")
				return subcommands.ExitFailure
			}
			v, err := strconv.ParseInt(f.Arg(1), 10, 32)
			if err != nil {
				if r.req.EndKey != nil {
					fmt.Fprintln(os.Stderr, "end key already set")
					return subcommands.ExitFailure
				}
				r.req.EndKey = &wrapperspb.StringValue{
					Value: f.Arg(1),
				}
				continue
			}
			r.req.Limit = &wrapperspb.UInt32Value{
				Value: uint32(v),
			}
		}
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Getrangekeys{
				Getrangekeys: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "getrangekeys")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 || f.Arg(0) == "" {
		fmt.Fprintln(os.Stderr, "must specify a tenant name")
		return subcommands.ExitFailure
	}
	r.req.Name = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Gettenant{
				Gettenant: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "gettenant")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "no additional arguments accepted")
		return subcommands.ExitFailure
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Getversion{
				Getversion: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "getversion")
}

type fdbCLIHelpCmd struct {
	req *pb.FDBCLIHelp
}

func (*fdbCLIHelpCmd) Name() string { return "help" }
func (*fdbCLIHelpCmd) Synopsis() string {
	return "The help command provides information on specific commands. Its syntax is help <TOPIC>, where <TOPIC> is any of the commands in this section, escaping, or options"
}
func (p *fdbCLIHelpCmd) Usage() string {
	return "help [TOPIC]"
}

func (r *fdbCLIHelpCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIHelp{}
}

func (r *fdbCLIHelpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	r.req.Options = append(r.req.Options, f.Args()...)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Help{
				Help: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "help")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	failed, address, all := false, false, false
	for _, opt := range f.Args() {
		if all {
			fmt.Fprintln(os.Stderr, "can't set any addresses after all is declared")
		}
		switch opt {
		case "failed":
			if failed {
				fmt.Fprintln(os.Stderr, "cannot specify failed more than once")
				return subcommands.ExitFailure
			}
			if address || all {
				fmt.Fprintln(os.Stderr, "cannot specify failed after listing addresses")
				return subcommands.ExitFailure
			}
			r.req.Failed = &wrapperspb.BoolValue{
				Value: true,
			}
			failed = true
		case "all":
			if all {
				fmt.Fprintln(os.Stderr, "cannot specify all more than once")
				return subcommands.ExitFailure
			}
			if address {
				fmt.Fprintln(os.Stderr, "cannot specify all and addresses together")
				return subcommands.ExitFailure
			}
			r.req.Request = &pb.FDBCLIInclude_All{
				All: true,
			}
			all = true
		default:
			if opt == "" {
				fmt.Fprintln(os.Stderr, "address cannot be blank")
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

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Include{
				Include: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "include")
}

type fdbCLIKillCmd struct {
	req *pb.FDBCLIKill
}

func (*fdbCLIKillCmd) Name() string { return "kill" }
func (*fdbCLIKillCmd) Synopsis() string {
	return "The kill command attempts to kill one or more processes in the cluster."
}
func (p *fdbCLIKillCmd) Usage() string {
	return `kill [list|address...]

NOTE: Internally this will be converted to kill; kill <address...> to do the required auto fill of addresses needed.
`
}

func (r *fdbCLIKillCmd) SetFlags(f *flag.FlagSet) {
	r.req = &pb.FDBCLIKill{}
}

func (r *fdbCLIKillCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	for _, opt := range f.Args() {
		if opt == "" {
			fmt.Fprintln(os.Stderr, "address cannot be blank")
			return subcommands.ExitFailure
		}
		r.req.Addresses = append(r.req.Addresses, opt)
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Kill{
				Kill: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "kill")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() > 3 {
		fmt.Fprintln(os.Stderr, "must specify at most begin/end and limit")
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
		if f.Arg(0) == "" || f.Arg(1) == "" {
			fmt.Fprintln(os.Stderr, "Begin and end must not be empty strings")
		}
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

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Listtenants{
				Listtenants: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "listtenants")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "no additional arguments accepted")
		return subcommands.ExitFailure
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Lock{
				Lock: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "lock")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() == 2 || f.NArg() > 3 {
		fmt.Fprintln(os.Stderr, `must specify either "on zoneid seconds" or "off"`)
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
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIMaintenance_Off{
			Off: &pb.FDBCLIMaintenanceOff{},
		}
	// on zoneid seconds
	case 3:
		if f.Arg(0) != "on" || f.Arg(1) == "" {
			fmt.Fprintln(os.Stderr, `3 argument case must be "on zoneid seconds"`)
			return subcommands.ExitFailure
		}
		v, err := strconv.ParseInt(f.Arg(2), 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't parse seconds: %v\n", err)
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIMaintenance_On{
			On: &pb.FDBCLIMaintenanceOn{
				Zoneid:  f.Arg(1),
				Seconds: uint32(v),
			},
		}
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Maintenance{
				Maintenance: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "maintenance")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() == 1 || f.NArg() > 3 {
		fmt.Fprintln(os.Stderr, "must specify either no options or 'on <option> [arg]' or 'off <option>'")
		return subcommands.ExitFailure
	}

	// Has to be 2 or 3 at this point based on above checks.
	switch f.Arg(0) {
	case "off", "on":
		// Just to validate.
	default:
		fmt.Fprintln(os.Stderr, "must specify on or off")
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

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Option{
				Option: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "option")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "must specify a profile option")
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
			fmt.Fprintln(os.Stderr, `profile client is of the form: profile client get|<set <rate|"default"> <size|"default">>`)
			return subcommands.ExitFailure
		}
	case "list":
		if f.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "profile list takes no arguments")
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
			// Don't care about value of next arg except shouldn't be blank
			if f.Arg(3) == "" {
				errCase = true
				break
			}
			for _, opt := range f.Args()[4:] {
				if opt == "" {
					errCase = true
					break
				}
				r.req.GetFlow().Processes = append(r.req.GetFlow().Processes, opt)
			}
		}
		if errCase {
			fmt.Fprintln(os.Stderr, "profile flow is of the form: profile flow run <duration> <filename> <process...>")
			return subcommands.ExitFailure
		}
	case "heap":
		if f.Arg(1) == "" {
			fmt.Fprintln(os.Stderr, "profile heap is of the form: profile heap <process>")
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIProfile_Heap{
			Heap: &pb.FDBCLIProfileActionHeap{
				Process: f.Arg(1),
			},
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown profile option")
		return subcommands.ExitFailure
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Profile{
				Profile: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "profile")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 2 || f.Arg(0) == "" || f.Arg(1) == "" {
		fmt.Fprintln(os.Stderr, "must specify key and value")
		return subcommands.ExitFailure

	}
	r.req.Key = f.Arg(0)
	r.req.Value = f.Arg(1)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Set{
				Set: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "set")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 0 && f.NArg() != 2 || (f.NArg() == 2 && (f.Arg(0) == "" || f.Arg(1) == "")) {
		fmt.Fprintln(os.Stderr, "must specify either no arguments or <address> <class>")
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

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Setclass{
				Setclass: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "setclass")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "must specify seconds")
		return subcommands.ExitFailure
	}
	v, err := strconv.ParseInt(f.Arg(0), 10, 32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid seconds: %v\n", err)
		return subcommands.ExitFailure
	}
	r.req.Seconds = uint32(v)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Sleep{
				Sleep: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "sleep")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "must specify no arguments or a style")
		return subcommands.ExitFailure
	}
	if f.NArg() == 1 {
		r.req.Style = &wrapperspb.StringValue{
			Value: f.Arg(0),
		}
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Status{
				Status: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "status")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "must specify on or off")
		return subcommands.ExitFailure
	}
	switch f.Arg(0) {
	case "on":
		if f.Arg(1) != "tag" || f.Arg(2) == "" {
			fmt.Fprintln(os.Stderr, "throttle on is of the form: throttle on tag <tag> [rate] [duration N<s|m|h|d>] [priority]")
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
						return subcommands.ExitFailure
					}
					r.req.GetOn().Duration = &wrapperspb.StringValue{
						Value: opt,
					}
				}
				// Everything else is a priority
				if r.req.GetOn().Priority != nil {
					fmt.Fprintln(os.Stderr, "can't set priority more than once for throttle on")
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
					return subcommands.ExitFailure
				}
				if r.req.GetOff().Tag != nil {
					fmt.Fprintln(os.Stderr, "throttle off can only specify one tag")
					return subcommands.ExitFailure
				}
				r.req.GetOff().Tag = &wrapperspb.StringValue{
					Value: f.Arg(i),
				}
			default:
				// Anything else is a priority
				if r.req.GetOff().Priority != nil {
					fmt.Fprintln(os.Stderr, "throttle off can only specify one priority")
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
			return subcommands.ExitFailure
		}
		r.req.Request = &pb.FDBCLIThrottle_Enable{
			Enable: &pb.FDBCLIThrottleActionEnable{},
		}
	case "disable":
		if f.NArg() != 2 || f.Arg(1) != "auto" {
			fmt.Fprintln(os.Stderr, "throttle disable is of the form: throttle disable auto")
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
			return subcommands.ExitFailure
		}
	default:
		fmt.Fprintln(os.Stderr, "must specify on, off, enable, disable, or list")
		return subcommands.ExitFailure
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Throttle{
				Throttle: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "throttle")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "triggerddteaminfolog takes no arguments")
		return subcommands.ExitFailure

	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Triggerddteaminfolog{
				Triggerddteaminfolog: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "triggerddteaminfolog")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "must specify uid")
		return subcommands.ExitFailure
	}
	r.req.Uid = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Unlock{
				Unlock: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "unlock")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "must specify tenant name")
		return subcommands.ExitFailure
	}
	r.req.Name = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Usetenant{
				Usetenant: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "usetenant")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "must specify mode")
		return subcommands.ExitFailure
	}
	r.req.Mode = f.Arg(0)

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Writemode{
				Writemode: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "writemode")
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
	state := args[0].(*util.ExecuteState)
	req := args[1].(*pb.FDBCLIRequest)

	c := pb.NewCLIClientProxy(state.Conn)

	if f.NArg() != 1 && f.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "must specify a valid tssq command")
		return subcommands.ExitFailure
	}
	switch f.Arg(0) {
	case "start":
		if f.NArg() != 2 {
			fmt.Fprintln(os.Stderr, "tssq start required a storage uid")
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
		fmt.Fprintln(os.Stderr, "unknown tssq command")
		return subcommands.ExitFailure
	}

	req.Request = &pb.FDBCLIRequest_Command{
		Command: &pb.FDBCLICommand{
			Command: &pb.FDBCLICommand_Tssq{
				Tssq: r.req,
			},
		},
	}

	return runFDBCLI(ctx, c, state, req, "tssq")
}
