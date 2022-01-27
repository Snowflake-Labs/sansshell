/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

// Package mtls facilitates Mutual TLS authentication for SansShell.
package mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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
func Register(name string, loader CredentialsLoader) error {
	loaderMu.Lock()
	defer loaderMu.Unlock()
	if loader == nil {
		return errors.New("loader cannot be nil")
	}
	if _, exists := loaders[name]; exists {
		return errors.New("duplicate registation of credentials loader with name: " + name)
	}
	loaders[name] = loader
	return nil
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

// Loaders returns the names of all currently registered CredentialLoader
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
