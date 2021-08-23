package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"path"

	"google.golang.org/grpc/credentials"
)

const (
	defaultClientCertPath = ".unshelled/client.pem"
	defaultClientKeyPath  = ".unshelled/client.key"
)

var (
	clientCertFile, clientKeyFile *string
)

func init() {
	cd, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	clientCertFile = flag.String("client-cert", path.Join(cd, defaultClientCertPath), "Path to this client's x509 cert, PEM format")
	clientKeyFile = flag.String("client-key", path.Join(cd, defaultClientKeyPath), "Path to this client's key")
}

// GetClientCredentials wraps LoadTLSClientKeys with convenient flag values and parsing
func GetClientCredentials() (credentials.TransportCredentials, error) {
	CAPool, err := LoadRootOfTrust(*rootCAFile) // defined in common.go
	if err != nil {
		return nil, err
	}
	clientOpt, err := LoadClientTLS(*clientCertFile, *clientKeyFile, CAPool)
	if err != nil {
		return nil, err
	}
	return clientOpt, nil
}

// LoadClientTLS reads the certificates and keys from disk at the supplied paths,
// and assembles them into a set of TransportCredentials for the gRPC client.
func LoadClientTLS(clientCertFile, clientKeyFile string, CAPool *x509.CertPool) (credentials.TransportCredentials, error) {
	// Read in client credentials
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read client credentials: %v\n", err)
		os.Exit(1)
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      CAPool,
	}), nil
}
