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
	"net"
	"sort"
	"sync"

	"google.golang.org/grpc/credentials"
)

var (
	loaderMu sync.RWMutex
	loaders  = make(map[string]CredentialsLoader)
)

// A CredentialsLoader loads mTLS credential data.
type CredentialsLoader interface {
	// LoadClientCA returns a CertPool which should be used by a server to
	// validate client certificates.
	// NOTE: The pool returned here will be the only pool used to validate certificates.
	//       Inclusion of system certificates should be done by copying from x509.SystemCertPool(),
	//       with any custom certificates appended.
	LoadClientCA(context.Context) (*x509.CertPool, error)

	// LoadRootCA returns a CertPool which should be used by clients to
	// validate server certificates.
	// NOTE: The pool returned here will be the only pool used to validate certificates.
	//       Inclusion of system certificates should be done by copying from x509.SystemCertPool(),
	//       with any custom certificates appended.
	LoadRootCA(context.Context) (*x509.CertPool, error)

	// LoadClientCertificates returns the certificate that should be presented
	// by the client when connecting to a server.
	LoadClientCertificate(context.Context) (tls.Certificate, error)

	// LoadServerCertificate returns the certificate that should be presented
	// by the server to incoming clients.
	LoadServerCertificate(context.Context) (tls.Certificate, error)

	// CertRefreshed indicates if internally any of the cert data has
	// been refreshed and should be reloaded. This will depend on the
	// implementation to support but allows for dynamic refresh of certificates
	// without a server restart.
	CertsRefreshed() bool
}

type WrappedTransportCredentials struct {
	creds      credentials.TransportCredentials
	loaderName string
	serverName string
	mtlsLoader CredentialsLoader
	loader     func(context.Context, string) (credentials.TransportCredentials, error)
}

func (w *WrappedTransportCredentials) checkRefresh() error {
	if w.mtlsLoader.CertsRefreshed() {
		newCreds, err := w.loader(context.Background(), w.loaderName)
		if err != nil {
			return err
		}
		w.creds = newCreds
		if w.serverName != "" {
			return w.creds.OverrideServerName(w.serverName) //nolint:staticcheck
		}
	}
	return nil
}

// ClientHandshake -- see credentials.ClientHandshake
func (w *WrappedTransportCredentials) ClientHandshake(ctx context.Context, s string, n net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if err := w.checkRefresh(); err != nil {
		return nil, nil, err
	}
	return w.creds.ClientHandshake(ctx, s, n)
}

// ServerHandshake -- see credentials.ServerHandshake
func (w *WrappedTransportCredentials) ServerHandshake(n net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if err := w.checkRefresh(); err != nil {
		return nil, nil, err
	}
	return w.creds.ServerHandshake(n)
}

// Info -- see credentials.Info
func (w *WrappedTransportCredentials) Info() credentials.ProtocolInfo {
	// We have no way to process an error with this API
	_ = w.checkRefresh()
	return w.creds.Info()
}

// Clone -- see credentials.Clone
func (w *WrappedTransportCredentials) Clone() credentials.TransportCredentials {
	// We have no way to process an error with this API
	_ = w.checkRefresh()
	wrapped := &WrappedTransportCredentials{
		creds:      w.creds.Clone(),
		loaderName: w.loaderName,
		loader:     w.loader,
		mtlsLoader: w.mtlsLoader,
	}
	return wrapped
}

// OverrideServerName -- see credentials.OverrideServerName
func (w *WrappedTransportCredentials) OverrideServerName(s string) error {
	if err := w.checkRefresh(); err != nil {
		return err
	}
	w.serverName = s
	return w.creds.OverrideServerName(s) //nolint:staticcheck
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
