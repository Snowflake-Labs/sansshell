package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/snowflakedb/unshelled/server"

	// Import the server modules you want to expose, they automatically register
	_ "github.com/snowflakedb/unshelled/services/healthcheck"
	_ "github.com/snowflakedb/unshelled/services/localfile"
)

var (
	//go:embed default-policy.rego
	defaultPolicy string
	hostport      = flag.String("hostport", "localhost:50042", "Where to listen for connections.")
	policyFlag    = flag.String("policy", defaultPolicy, "Local OPA policy governing access.  If empty, use builtin policy.")
	policyFile    = flag.String("policyFile", "", "Path to a file with an OPA policy.  If empty, uses --policy.")
)

func main() {
	flag.Parse()

	if *policyFlag != defaultPolicy && *policyFile != "" {
		fmt.Fprintf(os.Stderr, "Do not set both --policy and --policyFile.\n")
		os.Exit(2)
	}

	policy := *policyFlag
	if *policyFile != "" {
		pff, err := ioutil.ReadFile(*policyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(2)
		}
		policy = string(pff)
	}

	err := server.Serve(*hostport, policy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(3)
	}
}
