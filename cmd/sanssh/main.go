// Package main implements the SansShell CLI client.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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
	"github.com/Snowflake-Labs/sansshell/services/util"
)

var (
	defaultAddress = "localhost:50042"
	defaultTimeout = 3 * time.Second

	address    = flag.String("address", defaultAddress, "Address to contact sansshell-server. Implies a single RPC only and cannot be set with --targets.")
	proxyAddr  = flag.String("proxy", "", "Address to contact for proxy to sansshell-server. If blank a direct connection to --address will be made")
	timeout    = flag.Duration("timeout", defaultTimeout, "How long to wait for the command to complete")
	credSource = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))

	// targets will be bound to --targets for sending a single request to N nodes.
	targetsFlag stringSliceVar
	targets     []string
)

type stringSliceVar struct {
	target *[]string
}

func (s *stringSliceVar) Set(val string) error {
	*s.target = strings.Split(val, ",")
	return nil
}

func (s *stringSliceVar) String() string {
	if s.target == nil {
		return ""
	}
	return strings.Join(*s.target, ",")
}

func init() {
	targetsFlag.target = &targets
	flag.Var(&targetsFlag, "targets", "List of targets (separated by commas) to apply RPC against. Cannot be set with --address and --proxy must be set.")

	subcommands.ImportantFlag("address")
	subcommands.ImportantFlag("credential-source")
	subcommands.ImportantFlag("proxy")
	subcommands.ImportantFlag("targets")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
}

func main() {
	flag.Parse()

	// Bunch of flag sanity checking
	if *address == "" && len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "Must set one of --address or --targets")
		os.Exit(1)
	}
	if *address != "" && len(targets) > 0 {
		fmt.Fprintln(os.Stderr, "Can't set both --address and --targets together")
		os.Exit(1)
	}
	if len(targets) > 0 && *proxyAddr == "" {
		fmt.Fprintln(os.Stderr, "Can't set --targets without --proxy")
		os.Exit(1)
	}

	ctx := context.Background()
	creds, err := mtls.LoadClientCredentials(ctx, *credSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *address, err)
		os.Exit(1)
	}

	// Setup endpoints.
	var t []string
	if *address != "" {
		t = append(t, *address)
	}
	t = append(t, targets...)

	// Set up a connection to the sansshell-server (possibly via proxy).
	conn, err := proxy.Dial(*proxyAddr, t, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *address, err)
		os.Exit(1)
	}
	defer func() {
		if closer, ok := conn.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error closing connection - %v", err)
			}

		}
	}()

	state := &util.ExecuteState{
		Conn: conn,
		Out:  os.Stdout,
	}
	// Indicate to command whether we want N targets.
	if len(t) > 1 {
		state.MultipleTargets = true
	}

	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	// Invoke the subcommand, passing the dialed connection object
	// TODO(jchacon): Pass a struct instead of 3 args.
	os.Exit(int(subcommands.Execute(ctx, state)))
}
