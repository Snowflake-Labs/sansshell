// Package main implements the SansShell CLI client.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	// Import the raw command clients you want, they automatically register
	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	_ "github.com/Snowflake-Labs/sansshell/services/ansible/client"
	_ "github.com/Snowflake-Labs/sansshell/services/exec/client"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/client"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/client"
	_ "github.com/Snowflake-Labs/sansshell/services/packages/client"
	_ "github.com/Snowflake-Labs/sansshell/services/process/client"
	_ "github.com/Snowflake-Labs/sansshell/services/service/client"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

var (
	defaultAddress = "localhost:50042"
	defaultOutput  = "-"
	defaultTimeout = 3 * time.Second

	proxyAddr  = flag.String("proxy", "", "Address to contact for proxy to sansshell-server. If blank a direct connection to the first entry in --targets will be made")
	timeout    = flag.Duration("timeout", defaultTimeout, "How long to wait for the command to complete")
	credSource = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))

	// targets will be bound to --targets for sending a single request to N nodes.
	targetsFlag util.StringSliceFlag

	// outputs will be found to --outputs for directing output from a single request to N nodes.
	outputsFlag util.StringSliceFlag
)

func init() {
	targetsFlag.Set(defaultAddress)
	outputsFlag.Set(defaultOutput)

	flag.Var(&targetsFlag, "targets", "List of targets (separated by commas) to apply RPC against. If --proxy is not set must be one entry only.")
	flag.Var(&outputsFlag, "outputs", `List of output destinations (separated by commas) to direct output into. Use - to indicated stdout.
	NOTE: This must map 1:1 with --targets.`)

	subcommands.ImportantFlag("credential-source")
	subcommands.ImportantFlag("proxy")
	subcommands.ImportantFlag("targets")
	subcommands.ImportantFlag("outputs")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
}

func main() {
	flag.Parse()

	// Bunch of flag sanity checking
	if len(*targetsFlag.Target) == 0 {
		fmt.Fprintln(os.Stderr, "Must set --targets")
		os.Exit(1)
	}
	if len(*targetsFlag.Target) > 1 && *proxyAddr == "" {
		fmt.Fprintln(os.Stderr, "Can't set --targets to multiple entries without --proxy")
		os.Exit(1)
	}

	if len(*outputsFlag.Target) != len(*targetsFlag.Target) {
		fmt.Fprintln(os.Stderr, "--outputs and --targets must contain the same number of entries")
		os.Exit(1)
	}

	ctx := context.Background()
	creds, err := mtls.LoadClientCredentials(ctx, *credSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load creds from %s - %v", *credSource, err)
		os.Exit(1)
	}

	// Set up a connection to the sansshell-server (possibly via proxy).
	conn, err := proxy.Dial(*proxyAddr, *targetsFlag.Target, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *targetsFlag.Target, err)
		os.Exit(1)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing connection - %v", err)
		}
	}()

	state := &util.ExecuteState{
		Conn: conn,
	}
	for _, out := range *outputsFlag.Target {
		if out == "-" {
			state.Out = append(state.Out, os.Stdout)
			continue
		}
		file, err := os.Create(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't create output file %s - %v", out, err)
			os.Exit(1)
		}
		defer file.Close()
		state.Out = append(state.Out, file)
	}

	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	// Invoke the subcommand, passing the dialed connection object
	// TODO(jchacon): Pass a struct instead of 3 args.
	os.Exit(int(subcommands.Execute(ctx, state)))
}
