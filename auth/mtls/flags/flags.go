package flags

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"os"
	"path"

	"github.com/snowflakedb/unshelled/auth/mtls"
)

const (
	loaderName = "flags"

	defaultClientCertPath = ".unshelled/client.pem"
	defaultClientKeyPath  = ".unshelled/client.key"
	defaultServerCertPath = ".unshelled/leaf.pem"
	defaultServerKeyPath  = ".unshelled/leaf.key"
	defaultRootCAPath     = ".unshelled/root.pem"
)

var (
	clientCertFile, clientKeyFile string
	serverCertFile, serverKeyFile string
	rootCAFile                    string
)

func Name() string { return loaderName }

// flagLoader implements mtls.CredentialsLoader by reading files specified
// by command-line flags.
type flagLoader struct{}

func (f flagLoader) LoadClientCA(unused context.Context) (*x509.CertPool, error) {
	return mtls.LoadRootOfTrust(rootCAFile)
}

func (f flagLoader) LoadRootCA(unused context.Context) (*x509.CertPool, error) {
	return mtls.LoadRootOfTrust(rootCAFile)
}

func (f flagLoader) LoadClientCertificate(unused context.Context) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
}

func (f flagLoader) LoadServerCertificate(unused context.Context) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
}

func init() {
	cd, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	clientCertFile = path.Join(cd, defaultClientCertPath)
	clientKeyFile = path.Join(cd, defaultClientKeyPath)
	serverCertFile = path.Join(cd, defaultServerCertPath)
	serverKeyFile = path.Join(cd, defaultServerKeyPath)
	rootCAFile = path.Join(cd, defaultRootCAPath)

	flag.StringVar(&clientCertFile, "client-cert", clientCertFile, "Path to this client's x509 cert, PEM format")
	flag.StringVar(&clientKeyFile, "client-key", clientKeyFile, "Path to this client's key")
	flag.StringVar(&serverCertFile, "server-cert", serverCertFile, "Path to an x509 server cert, PEM format")
	flag.StringVar(&serverKeyFile, "server-key", serverKeyFile, "Path to the server's TLS key")
	flag.StringVar(&rootCAFile, "root-ca", rootCAFile, "The root of trust for remote identities, PEM format")

	mtls.Register(loaderName, flagLoader{})
}
