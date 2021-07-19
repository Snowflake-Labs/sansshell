// Package main implements the unshelled cli client.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	// Import the raw command clients you want, they automatically register
	_ "github.com/snowflakedb/unshelled/services/healthcheck/client"
	_ "github.com/snowflakedb/unshelled/services/localfile/client"
)

const (
	defaultAddress = "localhost:50042"
)

func main() {
	address := flag.String("address", defaultAddress, "Address to contact unshelled-server")
	subcommands.ImportantFlag("address")

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	flag.Parse()

	// Set up a connection to the unshelled-server.
	ctx := context.Background()
	// TODO: remove WithInsecure, pass a cert
	conn, err := grpc.Dial(*address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to %q: %v\n", *address, err)
		os.Exit(1)
	}
	defer conn.Close()

	// Invoke the subcommand, passing the dialed connection object
	os.Exit(int(subcommands.Execute(ctx, conn)))
}
