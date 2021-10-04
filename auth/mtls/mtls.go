// Package mtls facilitates Mutual TLS authentication for SansShell.
package mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sort"
	"sync"
)

var (
	loaderMu sync.RWMutex
	loaders  = make(map[string]CredentialsLoader)
)

// A CredentialsLoader loads mTLS credential data.
type CredentialsLoader interface {
	// LoadClientCA returns a CertPool which should be used by a server to
	// validate client certificates.
	LoadClientCA(context.Context) (*x509.CertPool, error)

	// LoadRootCA returns a CertPool which should be used by clients to
	// validate server certificates.
	LoadRootCA(context.Context) (*x509.CertPool, error)

	// LoadClientCertificates returns the certificate that should be presented
	// by the client when connecting to a server.
	LoadClientCertificate(context.Context) (tls.Certificate, error)

	// LoadServerCertificate returns the certificate that should be presented
	// by the server to incoming clients.
	LoadServerCertificate(context.Context) (tls.Certificate, error)
}

// Register associates a name with a mechanism for loading credentials.
// Implementations of CredentialsLoader will typically call Register
// during init()
func Register(name string, loader CredentialsLoader) {
	loaderMu.Lock()
	defer loaderMu.Unlock()
	if loader == nil {
		panic("loader cannot be nil")
	}
	if _, exists := loaders[name]; exists {
		panic("duplicate registation of credentials loader with name: " + name)
	}
	loaders[name] = loader
}

// Loader returns the CredentialsLoader associated with `name` or an
// error if no such implementation is registered.
func Loader(name string) (CredentialsLoader, error) {
	loaderMu.RLock()
	loader, ok := loaders[name]
	loaderMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown loader type %s", name)
	}
	return loader, nil
}

// Loaders() returns the names of all currently registered CredentialLoader
// implementations as a sorted list of strings.
func Loaders() []string {
	loaderMu.RLock()
	defer loaderMu.RUnlock()
	var out []string
	for l := range loaders {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// for testing
func unregisterAll() {
	loaderMu.Lock()
	loaders = make(map[string]CredentialsLoader)
	loaderMu.Unlock()
}
