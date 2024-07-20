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

	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/credentials"
)

// Metrics
var (
	clientHandshakeFailureMetrics = metrics.MetricDefinition{
		Name: "mtls_client_handshake_failure", Description: "failure during mtls client handshake",
	}
	serverHandshakeFailureMetrics = metrics.MetricDefinition{
		Name: "mtls_server_handshake_failure", Description: "failure during mtls server handshake",
	}
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

// WrappedTransportCredentials wraps a credentials.TransportCredentials and
// monitors any access to the underlying credentials are up to date by calling
// CertsRefreshed before continuing.
type WrappedTransportCredentials struct {
	// The loaderName, mtlsLoader and loader function
	// are only ever set on startup which is single threaded by definition so don't
	// need mutex protection.

	mu         sync.RWMutex
	creds      credentials.TransportCredentials // GUARDED_BY(mu)
	loaderName string
	serverName string // GUARDED_BY(mu)
	mtlsLoader CredentialsLoader
	loader     func(context.Context, string) (credentials.TransportCredentials, error)
	logger     logr.Logger
	recorder   metrics.MetricsRecorder
}

func (w *WrappedTransportCredentials) checkRefresh() error {
	if w.mtlsLoader.CertsRefreshed() {
		w.logger.Info("certs need reloading")
		return w.refreshNow()
	}
	return nil
}

func (w *WrappedTransportCredentials) refreshNow() error {
	// At least provide the logger we saved before we call into the loader
	// or we lose all debugability.
	ctx := context.Background()
	ctx = logr.NewContext(ctx, w.logger)
	newCreds, err := w.loader(ctx, w.loaderName)
	w.logger.V(1).Info("newCreds", "creds", newCreds, "error", err)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.creds = newCreds
	if w.serverName != "" {
		return w.creds.OverrideServerName(w.serverName) //nolint:staticcheck
	}
	return nil
}

// ClientHandshake -- see credentials.ClientHandshake
func (w *WrappedTransportCredentials) ClientHandshake(ctx context.Context, s string, n net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if err := w.checkRefresh(); err != nil {
		return nil, nil, err
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	conn, info, err := w.creds.ClientHandshake(ctx, s, n)
	if err != nil && w.recorder != nil {
		w.recorder.CounterOrLog(context.Background(), clientHandshakeFailureMetrics, 1)
	}
	return conn, info, err
}

// ServerHandshake -- see credentials.ServerHandshake
func (w *WrappedTransportCredentials) ServerHandshake(n net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if err := w.checkRefresh(); err != nil {
		return nil, nil, err
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	conn, info, err := w.creds.ServerHandshake(n)
	if err != nil && w.recorder != nil {
		w.recorder.CounterOrLog(context.Background(), serverHandshakeFailureMetrics, 1)
	}
	return conn, info, err
}

// Info -- see credentials.Info
func (w *WrappedTransportCredentials) Info() credentials.ProtocolInfo {
	// We have no way to process an error with this API
	_ = w.checkRefresh()
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.creds.Info()
}

// Clone -- see credentials.Clone
func (w *WrappedTransportCredentials) Clone() credentials.TransportCredentials {
	// We have no way to process an error with this API
	_ = w.checkRefresh()
	w.mu.Lock()
	defer w.mu.Unlock()
	wrapped := &WrappedTransportCredentials{
		creds:      w.creds.Clone(),
		loaderName: w.loaderName,
		loader:     w.loader,
		mtlsLoader: w.mtlsLoader,
		logger:     w.logger,
	}
	return wrapped
}

// OverrideServerName -- see credentials.OverrideServerName
func (w *WrappedTransportCredentials) OverrideServerName(s string) error {
	if err := w.checkRefresh(); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
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
