package mtls

import (
	"crypto/x509"
	"fmt"
	"os"
)

func LoadRootOfTrust(filename string) (*x509.CertPool, error) {
	// Read in the root of trust for client identities
	ca, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("could not read CA from %q: %w", filename, err)
	}
	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("could not add CA cert to pool: %w", err)
	}
	return capool, nil
}
