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

package mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/credentials"
)

// MultiIdentityCredentials implements credentials.TransportCredentials by
// dynamically selecting a client certificate during the TLS handshake based
// on the server's CertificateRequest.AcceptableCAs. This allows a single
// gRPC dial to present the correct identity without retry or fallback.
//
// Internally it delegates to credentials.NewTLS for the actual handshake,
// port stripping, ALPN negotiation, and TLSInfo construction. The only
// added behavior is certificate refresh and dynamic identity selection
// via GetClientCertificate.
type MultiIdentityCredentials struct {
	mu         sync.RWMutex
	identities []ClientIdentity
	creds      credentials.TransportCredentials
	serverName string
	rootCAs    *x509.CertPool

	// loaders and rootCALoader are resolved once at construction and
	// never change, matching WrappedTransportCredentials.mtlsLoader.
	loaders      []CredentialsLoader
	rootCALoader CredentialsLoader
	logger       logr.Logger
	recorder     metrics.MetricsRecorder
}

// NewMultiIdentityCredentials creates a TransportCredentials that dynamically
// selects among the provided identities during each TLS handshake.
//
// rootCALoader is the name of the registered CredentialsLoader whose
// LoadRootCA method provides the CertPool for server verification. It is
// reloaded on every certificate refresh so the trust roots stay current.
// When all identities share the same trust roots, pass any one identity's
// loader name. When identities have disjoint trust roots, register a loader
// that returns a pre-merged pool and pass its name.
//
// All loaders (both identity loaders and rootCALoader) are resolved from
// the global registry once at construction. If any loader name is not
// registered, construction fails immediately.
func NewMultiIdentityCredentials(identities []ClientIdentity, rootCALoader string, logger logr.Logger, recorder metrics.MetricsRecorder) (*MultiIdentityCredentials, error) {
	if len(identities) == 0 {
		return nil, errors.New("at least one identity is required")
	}
	if rootCALoader == "" {
		return nil, errors.New("rootCALoader name is required")
	}

	rootLoader, err := Loader(rootCALoader)
	if err != nil {
		return nil, fmt.Errorf("root CA loader %q: %w", rootCALoader, err)
	}
	rootCAs, err := rootLoader.LoadRootCA(context.Background())
	if err != nil {
		return nil, fmt.Errorf("loading initial root CAs from %q: %w", rootCALoader, err)
	}

	idLoaders := make([]CredentialsLoader, len(identities))
	for i := range identities {
		l, err := Loader(identities[i].LoaderName)
		if err != nil {
			return nil, fmt.Errorf("identity loader %q: %w", identities[i].LoaderName, err)
		}
		idLoaders[i] = l
	}

	m := &MultiIdentityCredentials{
		identities:   make([]ClientIdentity, len(identities)),
		loaders:      idLoaders,
		rootCALoader: rootLoader,
		rootCAs:      rootCAs,
		logger:       logger,
		recorder:     recorder,
	}
	copy(m.identities, identities)
	m.creds = m.buildInnerCreds(m.identities, rootCAs)

	for i := range m.identities {
		logIdentitySummary(logger, i, &m.identities[i])
	}
	logger.Info("multi-identity credentials initialized",
		"numIdentities", len(m.identities),
		"rootCALoader", rootCALoader,
	)

	return m, nil
}

// buildInnerCreds constructs a credentials.TransportCredentials backed by
// credentials.NewTLS with a GetClientCertificate callback that delegates
// to selectIdentity.
func (m *MultiIdentityCredentials) buildInnerCreds(identities []ClientIdentity, rootCAs *x509.CertPool) credentials.TransportCredentials {
	idsCopy := make([]ClientIdentity, len(identities))
	copy(idsCopy, identities)

	tlsCfg := &tls.Config{
		RootCAs:    rootCAs,
		MinVersion: tls.VersionTLS12,
		GetClientCertificate: func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			id, err := selectIdentity(cri, idsCopy, m.logger)
			if err != nil {
				return nil, err
			}
			return &id.Cert, nil
		},
	}
	return credentials.NewTLS(tlsCfg)
}

// refreshIfNeeded checks whether any loader signals new certificates and, if so,
// reloads all identities and the root CA pool. The check-then-reload is
// intentionally best-effort: concurrent callers may both observe CertsRefreshed
// and trigger duplicate reloads, but the final write is serialized by mu.
// This matches the pattern used by WrappedTransportCredentials.checkRefresh.
func (m *MultiIdentityCredentials) refreshIfNeeded(ctx context.Context) error {
	anyRefreshed := false
	for _, loader := range m.loaders {
		if loader.CertsRefreshed() {
			anyRefreshed = true
			break
		}
	}
	if !anyRefreshed {
		return nil
	}

	m.logger.Info("certs need reloading")

	m.mu.RLock()
	oldIdentities := m.identities
	m.mu.RUnlock()

	newIdentities := make([]ClientIdentity, len(m.loaders))
	for i, loader := range m.loaders {
		cert, err := loader.LoadClientCertificate(ctx)
		if err != nil {
			return fmt.Errorf("refreshing identity %q: %w", oldIdentities[i].LoaderName, err)
		}
		newIdentities[i] = ClientIdentity{LoaderName: oldIdentities[i].LoaderName, Cert: cert}
		logIdentitySummary(m.logger, i, &newIdentities[i])
	}

	newRootCAs, err := m.rootCALoader.LoadRootCA(ctx)
	if err != nil {
		return fmt.Errorf("refreshing root CAs: %w", err)
	}

	newCreds := m.buildInnerCreds(newIdentities, newRootCAs)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.creds = newCreds
	m.identities = newIdentities
	m.rootCAs = newRootCAs
	if m.serverName != "" {
		return m.creds.OverrideServerName(m.serverName) //nolint:staticcheck
	}
	return nil
}

// ClientHandshake refreshes certificates if needed, then delegates to the
// inner credentials.TransportCredentials built via credentials.NewTLS.
//
// Unlike WrappedTransportCredentials.ClientHandshake, refresh errors are
// logged and the handshake proceeds with existing certs. In multi-identity
// proxy scenarios availability is more important than cert freshness — a
// transient loader failure should not block all outbound connections.
func (m *MultiIdentityCredentials) ClientHandshake(ctx context.Context, serverName string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if err := m.refreshIfNeeded(ctx); err != nil {
		m.logger.Error(err, "cert refresh failed, using existing certs")
	}

	m.mu.RLock()
	creds := m.creds
	m.mu.RUnlock()

	conn, info, err := creds.ClientHandshake(ctx, serverName, rawConn)
	if err != nil {
		m.logger.Error(err, "TLS handshake failed",
			"serverName", serverName,
			"remoteAddr", rawConn.RemoteAddr().String(),
			"numIdentities", len(m.identities),
		)
		if m.recorder != nil {
			m.recorder.CounterOrLog(ctx, clientHandshakeFailureMetrics, 1)
		}
		return nil, nil, err
	}
	return conn, info, nil
}

// selectIdentity picks the first identity whose certificate is compatible
// with the server's CertificateRequest.
//
// Primary matching uses cri.SupportsCertificate, which checks signature
// algorithm compatibility and issuer matching (walking the full chain).
// See https://pkg.go.dev/crypto/tls#CertificateRequestInfo.SupportsCertificate
//
// If no identity passes SupportsCertificate (e.g. the server advertises
// signature schemes that don't cover all key types), a second pass falls
// back to raw issuer matching via chainMatchesAcceptableCAs. This ensures
// we still select the right identity when the CA trust relationship is
// correct but the signature scheme negotiation is overly restrictive.
//
// Falls back to identities[0] if the server sends an empty AcceptableCAs
// list, only one identity is configured, or no identity matches.
// Returns an error only if identities is empty.
func selectIdentity(cri *tls.CertificateRequestInfo, identities []ClientIdentity, logger logr.Logger) (*ClientIdentity, error) {
	if len(identities) == 0 {
		return nil, errors.New("no client identities available")
	}
	if len(cri.AcceptableCAs) == 0 || len(identities) == 1 {
		return &identities[0], nil
	}

	for i := range identities {
		if cri.SupportsCertificate(&identities[i].Cert) == nil {
			return &identities[i], nil
		}
	}

	acceptableSet := make(map[string]struct{}, len(cri.AcceptableCAs))
	for _, ca := range cri.AcceptableCAs {
		acceptableSet[string(ca)] = struct{}{}
	}
	for i := range identities {
		if chainMatchesAcceptableCAs(&identities[i], acceptableSet) {
			return &identities[i], nil
		}
	}

	logger.Info("no identity matched server's CertificateRequest, falling back to first identity")
	return &identities[0], nil
}

// chainMatchesAcceptableCAs checks whether any certificate in the identity's
// chain has a RawIssuer present in acceptableSet. Walking the full chain
// handles certs issued by intermediate CAs — the intermediate's RawIssuer
// is the root CA's subject DN, which is what servers typically advertise.
func chainMatchesAcceptableCAs(id *ClientIdentity, acceptableSet map[string]struct{}) bool {
	for _, derBytes := range id.Cert.Certificate {
		cert, err := x509.ParseCertificate(derBytes)
		if err != nil {
			continue
		}
		if _, ok := acceptableSet[string(cert.RawIssuer)]; ok {
			return true
		}
	}
	return false
}

func logIdentitySummary(logger logr.Logger, index int, id *ClientIdentity) {
	var leaf *x509.Certificate
	if id.Cert.Leaf != nil {
		leaf = id.Cert.Leaf
	} else if len(id.Cert.Certificate) > 0 {
		leaf, _ = x509.ParseCertificate(id.Cert.Certificate[0])
	}
	if leaf == nil {
		logger.Info("identity registered",
			"index", index,
			"identity", id.LoaderName,
			"certChainLen", len(id.Cert.Certificate),
		)
		return
	}
	logger.Info("identity registered",
		"index", index,
		"identity", id.LoaderName,
		"subject", leaf.Subject.String(),
		"issuer", leaf.Issuer.String(),
		"certChainLen", len(id.Cert.Certificate),
	)
}

// ServerHandshake is not supported — MultiIdentityCredentials is client-side only.
func (m *MultiIdentityCredentials) ServerHandshake(net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, errors.New("MultiIdentityCredentials does not support server-side handshake")
}

// Info delegates to the inner credentials.
func (m *MultiIdentityCredentials) Info() credentials.ProtocolInfo {
	m.mu.RLock()
	creds := m.creds
	m.mu.RUnlock()
	return creds.Info()
}

// Clone returns a deep copy of the credentials. The inner
// credentials.TransportCredentials is cloned via its own Clone method,
// preserving any OverrideServerName that was set.
func (m *MultiIdentityCredentials) Clone() credentials.TransportCredentials {
	m.mu.RLock()
	defer m.mu.RUnlock()
	identitiesCopy := make([]ClientIdentity, len(m.identities))
	copy(identitiesCopy, m.identities)
	return &MultiIdentityCredentials{
		identities:   identitiesCopy,
		loaders:      m.loaders,
		rootCALoader: m.rootCALoader,
		rootCAs:      m.rootCAs,
		serverName:   m.serverName,
		creds:        m.creds.Clone(),
		logger:       m.logger,
		recorder:     m.recorder,
	}
}

// OverrideServerName stores the override and delegates to the inner
// credentials so Info() reflects it. The stored name is re-applied
// whenever inner creds are rebuilt during certificate refresh.
//
// Deprecated: gRPC no longer calls OverrideServerName on credentials.
// Callers should use grpc.WithAuthority instead. This method is kept
// for interface compliance and for WrappedTransportCredentials parity
// (see mtls.go).
func (m *MultiIdentityCredentials) OverrideServerName(name string) error { //nolint:staticcheck
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serverName = name
	return m.creds.OverrideServerName(name) //nolint:staticcheck
}
