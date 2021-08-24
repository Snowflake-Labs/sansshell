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
	defaultServerCertPath = ".unshelled/leaf.pem"
	defaultServerKeyPath  = ".unshelled/leaf.key"
)

var (
	serverCertFile, serverKeyFile string
)

func init() {
	cd, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	serverCertFile = path.Join(cd, defaultServerCertPath)
	serverKeyFile = path.Join(cd, defaultServerKeyPath)
	flag.StringVar(&serverCertFile, "server-cert", serverCertFile, "Path to an x509 server cert, PEM format")
	flag.StringVar(&serverKeyFile, "server-key", serverKeyFile, "Path to the server's TLS key")
}

// GetServerCredentials wraps LoadServerTLS with convenient flag values and parsing
func GetServerCredentials() (credentials.TransportCredentials, error) {
	CAPool, err := GetCACredentials() // defined in common.go
	if err != nil {
		return nil, err
	}
	serverOpt, err := LoadServerTLS(serverCertFile, serverKeyFile, CAPool)
	if err != nil {
		return nil, err
	}
	return serverOpt, nil
}

// LoadServerTLS reads the certificates and keys from disk at the supplied paths,
// and assembles them into a set of TransportCredentials for the gRPC server.
func LoadServerTLS(clientCertFile, clientKeyFile string, CAPool *x509.CertPool) (credentials.TransportCredentials, error) {
	// Read in client credentials
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("reading client credentials: %v", err)
	}

	return credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    CAPool,
		MinVersion:   tls.VersionTLS13,
	}), nil
}
