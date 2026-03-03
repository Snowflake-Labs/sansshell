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
	"net"
	"sync"

	"google.golang.org/grpc/credentials"
)

// Identity represents a single client identity (certificate + trusted CA pool)
// that can be presented during a TLS handshake.
type Identity struct {
	Name    string
	Cert    tls.Certificate
	RootCAs *x509.CertPool
}

// MultiIdentityCredentials implements credentials.TransportCredentials by
// dynamically selecting a client certificate during the TLS handshake based
// on the server's CertificateRequest.AcceptableCAs. This allows a single
// gRPC dial to present the correct identity without retry or fallback.
//
// All identities are assumed to share a RootCAs pool that already contains
// every CA needed to verify any target server. The pools from all identities
// are merged at construction time and used for standard Go TLS server
// verification.
type MultiIdentityCredentials struct {
	mu          sync.RWMutex
	identities  []Identity
	loaderNames []string
	rootCAs     *x509.CertPool
	serverName  string
}

// NewMultiIdentityCredentials creates a TransportCredentials that dynamically
// selects among the provided identities during each TLS handshake.
// At least one identity must be provided. When only one identity is given,
// behavior is equivalent to standard single-identity TLS credentials.
func NewMultiIdentityCredentials(identities []Identity) (*MultiIdentityCredentials, error) {
	if len(identities) == 0 {
		return nil, errors.New("at least one identity is required")
	}
	names := make([]string, len(identities))
	for i := range identities {
		names[i] = identities[i].Name
	}
	m := &MultiIdentityCredentials{
		identities:  make([]Identity, len(identities)),
		loaderNames: names,
		rootCAs:     identities[0].RootCAs,
	}
	copy(m.identities, identities)
	return m, nil
}

func (m *MultiIdentityCredentials) refreshIfNeeded(ctx context.Context) {
	for i, name := range m.loaderNames {
		loader, err := Loader(name)
		if err != nil {
			continue
		}
		if !loader.CertsRefreshed() {
			continue
		}
		id, err := LoadClientIdentity(ctx, name)
		if err != nil {
			continue
		}
		m.mu.Lock()
		m.identities[i] = id
		m.rootCAs = id.RootCAs
		m.mu.Unlock()
	}
}

// ClientHandshake performs the client-side TLS handshake. It uses
// GetClientCertificate to pick the right client certificate based on the
// server's CertificateRequest.AcceptableCAs. Server certificate verification
// uses the merged RootCAs pool via standard Go TLS verification.
func (m *MultiIdentityCredentials) ClientHandshake(ctx context.Context, serverName string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	m.refreshIfNeeded(ctx)

	m.mu.RLock()
	overrideName := m.serverName
	identities := m.identities
	rootCAs := m.rootCAs
	m.mu.RUnlock()

	if overrideName != "" {
		serverName = overrideName
	}

	tlsCfg := &tls.Config{
		ServerName: serverName,
		RootCAs:    rootCAs,
		MinVersion: tls.VersionTLS12,
		GetClientCertificate: func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			idx := selectIdentity(cri, identities)
			cert := identities[idx].Cert
			return &cert, nil
		},
	}

	conn := tls.Client(rawConn, tlsCfg)
	if err := conn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, nil, err
	}

	info := credentials.TLSInfo{
		State:          conn.ConnectionState(),
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
	}
	return conn, info, nil
}

// selectIdentity picks the first identity whose issuing CA appears in the
// server's AcceptableCAs list. Falls back to index 0 if the server sends
// an empty list or no match is found.
func selectIdentity(cri *tls.CertificateRequestInfo, identities []Identity) int {
	if len(cri.AcceptableCAs) == 0 || len(identities) <= 1 {
		return 0
	}
	acceptableSet := make(map[string]struct{}, len(cri.AcceptableCAs))
	for _, ca := range cri.AcceptableCAs {
		acceptableSet[string(ca)] = struct{}{}
	}
	for i := range identities {
		issuer := leafIssuer(&identities[i])
		if issuer == nil {
			continue
		}
		if _, ok := acceptableSet[string(issuer)]; ok {
			return i
		}
	}
	return 0
}

func leafIssuer(id *Identity) []byte {
	if id.Cert.Leaf != nil {
		return id.Cert.Leaf.RawIssuer
	}
	if len(id.Cert.Certificate) > 0 {
		leaf, err := x509.ParseCertificate(id.Cert.Certificate[0])
		if err == nil {
			return leaf.RawIssuer
		}
	}
	return nil
}

// ServerHandshake is not supported — MultiIdentityCredentials is client-side only.
func (m *MultiIdentityCredentials) ServerHandshake(net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, errors.New("MultiIdentityCredentials does not support server-side handshake")
}

// Info returns protocol info.
func (m *MultiIdentityCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "tls",
		SecurityVersion:  "1.2",
		ServerName:       m.serverName,
	}
}

// Clone returns a deep copy of the credentials.
func (m *MultiIdentityCredentials) Clone() credentials.TransportCredentials {
	m.mu.RLock()
	defer m.mu.RUnlock()
	identitiesCopy := make([]Identity, len(m.identities))
	copy(identitiesCopy, m.identities)
	namesCopy := make([]string, len(m.loaderNames))
	copy(namesCopy, m.loaderNames)
	return &MultiIdentityCredentials{
		identities:  identitiesCopy,
		loaderNames: namesCopy,
		rootCAs:     m.rootCAs,
		serverName:  m.serverName,
	}
}

// OverrideServerName sets the server name used for TLS verification.
func (m *MultiIdentityCredentials) OverrideServerName(name string) error { //nolint:staticcheck
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serverName = name
	return nil
}
