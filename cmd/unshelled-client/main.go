// Package main implements the unshelled cli client.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	// Import the raw command clients you want, they automatically register
	"github.com/snowflakedb/unshelled/auth/mtls"
	_ "github.com/snowflakedb/unshelled/services/exec/client"
	_ "github.com/snowflakedb/unshelled/services/healthcheck/client"
	_ "github.com/snowflakedb/unshelled/services/localfile/client"
)

var (
	defaultAddress = "localhost:50042"
	defaultTimeout = 3 * time.Second
)

func main() {
	address := flag.String("address", defaultAddress, "Address to contact unshelled-server")
	timeout := flag.Duration("timeout", defaultTimeout, "How long to wait for the command to complete")

	subcommands.ImportantFlag("address")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	flag.Parse()

	creds, err := mtls.GetClientCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *address, err)
		os.Exit(1)
	}

	// Set up a connection to the unshelled-server.
	conn, err := grpc.Dial(*address, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *address, err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Invoke the subcommand, passing the dialed connection object
	os.Exit(int(subcommands.Execute(ctx, conn)))
}
