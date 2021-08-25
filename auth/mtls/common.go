// Package mtls facilitates Mutual TLS authentication for Unshelled.
package mtls

import (
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
)

const (
	defaultRootCAPath = ".unshelled/root.pem"
)

var (
	rootCAFile string
)

func init() {
	cd, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	rootCAFile = path.Join(cd, defaultRootCAPath)
	flag.StringVar(&rootCAFile, "root-ca", rootCAFile, "The root of trust for remote identities, PEM format")
}

// GetCACredentials wraps LoadRootOfTrust with the default flag value
func GetCACredentials() (*x509.CertPool, error) {
	return LoadRootOfTrust(rootCAFile)
}

func LoadRootOfTrust(filename string) (*x509.CertPool, error) {
	// Read in the root of trust for client identities
	ca, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("could not read CA from %q: %w", filename, err)
	}
	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("could not add CA cert to pool: %w", err)
	}
	return capool, nil
}
