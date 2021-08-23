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
	rootCAFile *string
)

func init() {
	cd, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	rootCAFile = flag.String("root-ca", path.Join(cd, defaultRootCAPath), "The root of trust for remote identities, PEM format")
}

func LoadRootOfTrust(filename string) (*x509.CertPool, error) {
	// Read in the root of trust for client identities
	ca, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("could not read CA from %q: %v", filename, err)
	}
	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("could not add CA cert to pool: %v", err)
	}
	return capool, nil
}
