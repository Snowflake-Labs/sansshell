package main

import (
	_ "embed"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/snowflakedb/unshelled/server"

	// Import the server modules you want to expose, they automatically register
	_ "github.com/snowflakedb/unshelled/services/exec"
	_ "github.com/snowflakedb/unshelled/services/healthcheck"
	_ "github.com/snowflakedb/unshelled/services/localfile"
)

const (
	defaultServerCertPath = ".unshelled/leaf.pem"
	defaultServerKeyPath  = ".unshelled/leaf.key"
	defaultRootCAPath     = ".unshelled/root.pem"
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
	cd, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	rootCAFile := flag.String("root-ca", path.Join(cd, defaultRootCAPath), "Path to a trusted server CA or Cert, PEM format")
	serverCertFile := flag.String("server-cert", path.Join(cd, defaultServerCertPath), "Path to an x509 server cert, PEM format")
	serverKeyFile := flag.String("server-key", path.Join(cd, defaultServerKeyPath), "Path to the server's TLS key")
	flag.Parse()

	policy := choosePolicy()

	creds, err := server.LoadTLSKeys(*rootCAFile, *serverCertFile, *serverKeyFile)
	if err != nil {
		log.Fatal(err)
	}

	if err := server.Serve(*hostport, creds, policy); err != nil {
		log.Fatal(err)
	}
}
