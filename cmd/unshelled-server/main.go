package main

import (
	_ "embed"
	"flag"
	"io/ioutil"
	"log"

	"github.com/snowflakedb/unshelled/server"

	// Import the server modules you want to expose, they automatically register
	_ "github.com/snowflakedb/unshelled/services/exec"
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

// choosePolicy selects an OPA policy based on the flags, or calls log.Fatal if
// an invalid combination is provided.
func choosePolicy() string {
	if *policyFlag != defaultPolicy && *policyFile != "" {
		log.Fatal("Do not set both --policy and --policyFile.")
	}

	var policy string
	if *policyFile != "" {
		pff, err := ioutil.ReadFile(*policyFile)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Using policy from --policyFlag")
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

	err := server.Serve(*hostport, policy)
	if err != nil {
		log.Fatal(err)
	}
}
