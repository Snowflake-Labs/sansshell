// Package main implements the unshelled cli client.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/google/subcommands"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	// Import the raw command clients you want, they automatically register
	_ "github.com/snowflakedb/unshelled/services/exec/client"
	_ "github.com/snowflakedb/unshelled/services/healthcheck/client"
	_ "github.com/snowflakedb/unshelled/services/localfile/client"
)

var (
	defaultAddress        = "localhost:50042"
	defaultTimeout        = 3 * time.Second
	defaultClientCertPath = ".unshelled/client.pem"
	defaultClientKeyPath  = ".unshelled/client.key"
	defaultRootCAPath     = ".unshelled/root.pem"
)

func main() {
	cd, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	address := flag.String("address", defaultAddress, "Address to contact unshelled-server")
	timeout := flag.Duration("timeout", defaultTimeout, "How long to wait for the command to complete")
	rootCertFile := flag.String("root-ca", path.Join(cd, defaultRootCAPath), "The root of trust for server identities, PEM format")
	clientCertFile := flag.String("client-cert", path.Join(cd, defaultClientCertPath), "Path to this client's x509 cert, PEM format")
	clientKeyFile := flag.String("client-key", path.Join(cd, defaultClientKeyPath), "Path to this client's key")

	subcommands.ImportantFlag("address")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	flag.Parse()

	// Read in the root of trust for client identities
	ca, err := ioutil.ReadFile(*rootCertFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read server CA from %q: %v\n", *rootCertFile, err)
		os.Exit(1)
	}
	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		fmt.Fprintf(os.Stderr, "Could not add CA cert: %v\n", err)
		os.Exit(1)
	}

	// Read in client credentials
	cert, err := tls.LoadX509KeyPair(*clientCertFile, *clientKeyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read client credentials: %v\n", err)
		os.Exit(1)
	}

	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      capool,
	})

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
