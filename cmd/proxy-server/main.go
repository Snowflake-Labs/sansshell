package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"

	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/proxy/server"

	// Import services here to make them proxy-able
	_ "github.com/Snowflake-Labs/sansshell/services/ansible"
	_ "github.com/Snowflake-Labs/sansshell/services/exec"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile"
	_ "github.com/Snowflake-Labs/sansshell/services/packages"
)

func main() {
	hostport := flag.String("hostport", "localhost:50043", "Where to listen for connections.")
	credSource := flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS creds (one of [%s])", strings.Join(mtls.Loaders(), ",")))

	flag.Parse()

	ctx := context.Background()

	serverCreds, err := mtls.LoadServerCredentials(ctx, *credSource)
	if err != nil {
		log.Fatal(err)
	}
	g := grpc.NewServer(grpc.Creds(serverCreds))
	clientCreds, err := mtls.LoadClientCredentials(ctx, *credSource)
	if err != nil {
		log.Fatal(err)
	}
	targetDialer := server.NewDialer(grpc.WithTransportCredentials(clientCreds))
	server := server.New(targetDialer)
	server.Register(g)

	lis, err := net.Listen("tcp", *hostport)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
