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
)

var (
	defaultAddress = "localhost:50042"
	defaultTimeout = 3 * time.Second

	address    = flag.String("address", defaultAddress, "Address to contact sansshell-server")
	proxyAddr  = flag.String("proxy", "", "Address to contact for proxy to sansshell-server. If blank a direct connection to --address will be made")
	timeout    = flag.Duration("timeout", defaultTimeout, "How long to wait for the command to complete")
	credSource = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
)

func main() {
	subcommands.ImportantFlag("address")
	subcommands.ImportantFlag("credential-source")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	flag.Parse()

	ctx := context.Background()

	creds, err := mtls.LoadClientCredentials(ctx, *credSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *address, err)
		os.Exit(1)
	}

	// Set up a connection to the sansshell-server.
	conn, err := proxy.Dial(*proxyAddr, []string{*address}, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *address, err)
		os.Exit(1)
	}
	if *proxyAddr == "" {
		defer conn.(*grpc.ClientConn).Close()
	} else {
		defer conn.(*proxy.ProxyConn).Close()
	}

	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	// Invoke the subcommand, passing the dialed connection object
	os.Exit(int(subcommands.Execute(ctx, conn)))
}
