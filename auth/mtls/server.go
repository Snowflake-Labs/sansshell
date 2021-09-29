package mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"google.golang.org/grpc/credentials"
)

// GetServerCredentials returns transport credentials for an unshelled server as retrieved from the
// specified `loaderName`
func LoadServerCredentials(ctx context.Context, loaderName string) (credentials.TransportCredentials, error) {
	loader, err := Loader(loaderName)
	if err != nil {
		return nil, err
	}

	pool, err := loader.LoadClientCA(ctx)
	if err != nil {
		return nil, err
	}
	cert, err := loader.LoadServerCertificate(ctx)
	if err != nil {
		return nil, err
	}
	return NewServerCredentials(cert, pool), nil
}

// NewServerCredentials creates transport credentials for an unshelled server.
func NewServerCredentials(cert tls.Certificate, CAPool *x509.CertPool) credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    CAPool,
		MinVersion:   tls.VersionTLS13,
	})
}

// LoadServerTLS reads the certificates and keys from disk at the supplied paths,
// and assembles them into a set of TransportCredentials for the gRPC server.
func LoadServerTLS(clientCertFile, clientKeyFile string, CAPool *x509.CertPool) (credentials.TransportCredentials, error) {
	// Read in client credentials
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("reading client credentials: %w", err)
	}
	return NewServerCredentials(cert, CAPool), nil
}
