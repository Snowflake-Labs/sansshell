//go:build go1.16
// +build go1.16

// Package main implements the SansShell server.
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	mtlsFlags "github.com/Snowflake-Labs/sansshell/auth/mtls/flags"
	"github.com/Snowflake-Labs/sansshell/server"

	// Import the server modules you want to expose, they automatically register
	_ "github.com/Snowflake-Labs/sansshell/services/ansible/server"
	_ "github.com/Snowflake-Labs/sansshell/services/exec/server"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/server"
	_ "github.com/Snowflake-Labs/sansshell/services/packages/server"
	_ "github.com/Snowflake-Labs/sansshell/services/process/server"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string
	hostport      = flag.String("hostport", "localhost:50042", "Where to listen for connections.")
	policyFlag    = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile    = flag.String("policy-file", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
	credSource    = flag.String("credential-source", mtlsFlags.Name(), fmt.Sprintf("Method used to obtain mTLS credentials (one of [%s])", strings.Join(mtls.Loaders(), ",")))
)

// choosePolicy selects an OPA policy based on the flags, or calls log.Fatal if
// an invalid combination is provided.
func choosePolicy() string {
	if *policyFlag != defaultPolicy && *policyFile != "" {
		log.Fatal("Do not set both --policy and --policy-file.")
	}

	var policy string
	if *policyFile != "" {
		pff, err := os.ReadFile(*policyFile)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Using policy from --policy-file")
		policy = string(pff)
	} else {
		if *policyFlag != defaultPolicy {
			log.Println("Using policy from --policy")
			policy = *policyFlag
		} else {
			log.Println("Using built-in policy")
			policy = defaultPolicy
		}
	}
	return policy
}

func main() {
	flag.Parse()

	policy := choosePolicy()
	ctx := context.Background()

	creds, err := mtls.LoadServerCredentials(ctx, *credSource)
	if err != nil {
		log.Fatal(err)
	}

	if err := server.Serve(*hostport, creds, policy); err != nil {
		log.Fatal(err)
	}
}
