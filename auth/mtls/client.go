package mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"google.golang.org/grpc/credentials"
)

// GetClientCredentials returns transport credentials for SansShell clients,
// based on the provided `loaderName`
func LoadClientCredentials(ctx context.Context, loaderName string) (credentials.TransportCredentials, error) {
	loader, err := Loader(loaderName)
	if err != nil {
		return nil, err
	}

	pool, err := loader.LoadRootCA(ctx)
	if err != nil {
		return nil, err
	}
	cert, err := loader.LoadClientCertificate(ctx)
	if err != nil {
		return nil, err
	}
	return NewClientCredentials(cert, pool), nil
}

// NewClientCredentials returns transport credentials for SansShell clients.
func NewClientCredentials(cert tls.Certificate, CAPool *x509.CertPool) credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      CAPool,
		MinVersion:   tls.VersionTLS13,
	})
}

// LoadClientTLS reads the certificates and keys from disk at the supplied paths,
// and assembles them into a set of TransportCredentials for the gRPC client.
func LoadClientTLS(clientCertFile, clientKeyFile string, CAPool *x509.CertPool) (credentials.TransportCredentials, error) {
	// Read in client credentials
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read client credentials: %w", err)
	}
	return NewClientCredentials(cert, CAPool), nil
}
