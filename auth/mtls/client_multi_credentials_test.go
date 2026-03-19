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
	"net"
	"sync"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func loadClientIdentity(t *testing.T, certFile, keyFile, name string) ClientIdentity {
	t.Helper()
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	testutil.FatalOnErr("LoadX509KeyPair", err, t)
	return ClientIdentity{LoaderName: name, Cert: cert}
}

func acmeIdentity(t *testing.T) ClientIdentity {
	t.Helper()
	return loadClientIdentity(t, "testdata/client.pem", "testdata/client.key", "acme")
}

func otherIdentity(t *testing.T) ClientIdentity {
	t.Helper()
	return loadClientIdentity(t, "testdata/client2.pem", "testdata/client2.key", "other")
}

func chainIdentity(t *testing.T) ClientIdentity {
	t.Helper()
	return loadClientIdentity(t, "testdata/client3.pem", "testdata/client3.key", "chain")
}

func parseLeaf(id *ClientIdentity) *x509.Certificate {
	if id.Cert.Leaf != nil {
		return id.Cert.Leaf
	}
	if len(id.Cert.Certificate) > 0 {
		leaf, err := x509.ParseCertificate(id.Cert.Certificate[0])
		if err == nil {
			return leaf
		}
	}
	return nil
}

// mergedRootLoader returns the merged pool (testdata/roots_merged.pem)
// that trusts all test CAs. Used as rootCALoader for multi-identity tests.
type mergedRootLoader struct{ simpleLoader }

func (mergedRootLoader) LoadRootCA(context.Context) (*x509.CertPool, error) {
	return LoadRootOfTrust("testdata/roots_merged.pem")
}

func testLogger(t *testing.T) logr.Logger {
	t.Helper()
	return testr.New(t)
}

func registerTestLoaders(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := Register(n, &simpleLoader{name: n}); err != nil {
			t.Logf("Register %q: %v (may already exist)", n, err)
		}
	}
}

// testCRI builds a CertificateRequestInfo with the given AcceptableCAs and
// realistic Version/SignatureSchemes so that cri.SupportsCertificate works
// for both ECDSA and RSA test certs.
func testCRI(acceptableCAs ...[]byte) *tls.CertificateRequestInfo {
	return &tls.CertificateRequestInfo{
		Version:       tls.VersionTLS13,
		AcceptableCAs: acceptableCAs,
		SignatureSchemes: []tls.SignatureScheme{
			tls.ECDSAWithP256AndSHA256,
			tls.PSSWithSHA256,
		},
	}
}

// tlsServer starts a TLS server that requires client certs signed by clientCA.
// It completes the handshake on each connection before closing it.
func tlsServer(t *testing.T, serverCert tls.Certificate, clientCA *x509.CertPool) net.Listener {
	t.Helper()
	cfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    clientCA,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2"},
	}
	lis, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	testutil.FatalOnErr("tls.Listen", err, t)
	t.Cleanup(func() { lis.Close() })

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			if tlsConn, ok := conn.(*tls.Conn); ok {
				_ = tlsConn.Handshake()
			}
			conn.Close()
		}
	}()
	return lis
}

// ---------------------------------------------------------------------------
// TestNewMultiIdentityCredentials — constructor validation
// ---------------------------------------------------------------------------

func TestNewMultiIdentityCredentials(t *testing.T) {
	unregisterAll()
	registerTestLoaders(t, "simple", "acme", "other")

	t.Run("zero identities", func(t *testing.T) {
		_, err := NewMultiIdentityCredentials(nil, "simple", testLogger(t), nil)
		testutil.FatalOnNoErr("zero identities", err, t)
	})

	t.Run("empty rootCALoader", func(t *testing.T) {
		id := acmeIdentity(t)
		_, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "", testLogger(t), nil)
		testutil.FatalOnNoErr("empty rootCALoader", err, t)
	})

	t.Run("unknown rootCALoader", func(t *testing.T) {
		id := acmeIdentity(t)
		_, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "nonexistent", testLogger(t), nil)
		testutil.FatalOnNoErr("unknown rootCALoader", err, t)
	})

	t.Run("unknown identity loader", func(t *testing.T) {
		id := ClientIdentity{LoaderName: "bogus", Cert: acmeIdentity(t).Cert}
		_, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "simple", testLogger(t), nil)
		testutil.FatalOnNoErr("unknown identity loader", err, t)
	})

	t.Run("single identity", func(t *testing.T) {
		id := acmeIdentity(t)
		m, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "simple", testLogger(t), nil)
		testutil.FatalOnErr("single identity", err, t)
		if len(m.identities) != 1 {
			t.Fatalf("got %d identities, want 1", len(m.identities))
		}
		if m.rootCAs == nil {
			t.Fatal("rootCAs should not be nil")
		}
		if m.rootCALoader == nil {
			t.Fatal("rootCALoader should not be nil")
		}
	})

	t.Run("two identities", func(t *testing.T) {
		ids := []ClientIdentity{acmeIdentity(t), acmeIdentity(t)}
		ids[1].LoaderName = "other"
		m, err := NewMultiIdentityCredentials(ids, "simple", testLogger(t), nil)
		testutil.FatalOnErr("two identities", err, t)
		if len(m.identities) != 2 {
			t.Fatalf("got %d identities, want 2", len(m.identities))
		}
		if m.identities[0].LoaderName != "acme" || m.identities[1].LoaderName != "other" {
			t.Fatalf("identities = [%s, %s], want [acme, other]", m.identities[0].LoaderName, m.identities[1].LoaderName)
		}
	})

	t.Run("mutation isolation", func(t *testing.T) {
		ids := []ClientIdentity{acmeIdentity(t)}
		m, err := NewMultiIdentityCredentials(ids, "simple", testLogger(t), nil)
		testutil.FatalOnErr("new", err, t)
		ids[0].LoaderName = "mutated"
		if m.identities[0].LoaderName == "mutated" {
			t.Fatal("constructor did not copy identities slice")
		}
	})
}

// ---------------------------------------------------------------------------
// TestSelectIdentity — identity selection by AcceptableCAs
// ---------------------------------------------------------------------------

func TestSelectIdentity(t *testing.T) {
	idAcme := acmeIdentity(t)
	idOther := otherIdentity(t)
	identities := []ClientIdentity{idAcme, idOther}
	logger := testLogger(t)

	acmeLeaf := parseLeaf(&idAcme)
	otherLeaf := parseLeaf(&idOther)
	if acmeLeaf == nil || otherLeaf == nil {
		t.Fatal("parseLeaf returned nil for test identities")
	}
	acmeIssuer := acmeLeaf.RawIssuer
	otherIssuer := otherLeaf.RawIssuer

	t.Run("empty AcceptableCAs returns first", func(t *testing.T) {
		got, err := selectIdentity(testCRI(), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "acme" {
			t.Fatalf("got %q, want acme", got.LoaderName)
		}
	})

	t.Run("single identity always returns it", func(t *testing.T) {
		got, err := selectIdentity(testCRI(otherIssuer), []ClientIdentity{idAcme}, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "acme" {
			t.Fatalf("got %q, want acme", got.LoaderName)
		}
	})

	t.Run("matches acme identity", func(t *testing.T) {
		got, err := selectIdentity(testCRI(acmeIssuer), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "acme" {
			t.Fatalf("got %q, want acme", got.LoaderName)
		}
	})

	t.Run("matches other identity", func(t *testing.T) {
		got, err := selectIdentity(testCRI(otherIssuer), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "other" {
			t.Fatalf("got %q, want other", got.LoaderName)
		}
	})

	t.Run("multiple acceptable CAs picks first match", func(t *testing.T) {
		got, err := selectIdentity(testCRI(otherIssuer, acmeIssuer), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "acme" {
			t.Fatalf("got %q, want acme (first matching identity)", got.LoaderName)
		}
	})

	t.Run("no match falls back to first", func(t *testing.T) {
		got, err := selectIdentity(testCRI([]byte("unknown-ca")), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "acme" {
			t.Fatalf("got %q, want acme (fallback)", got.LoaderName)
		}
	})
}

// ---------------------------------------------------------------------------
// TestSelectIdentity_ChainTraversal — certs issued by intermediate CAs
// ---------------------------------------------------------------------------

func TestSelectIdentity_ChainTraversal(t *testing.T) {
	idAcme := acmeIdentity(t)
	idChain := chainIdentity(t)
	logger := testLogger(t)

	if len(idChain.Cert.Certificate) < 2 {
		t.Fatalf("chain identity should have >=2 certs in chain, got %d", len(idChain.Cert.Certificate))
	}

	root3Pool, err := LoadRootOfTrust("testdata/root3.pem")
	testutil.FatalOnErr("LoadRootOfTrust root3", err, t)
	root3Subjects := root3Pool.Subjects() //nolint:staticcheck
	if len(root3Subjects) == 0 {
		t.Fatal("root3 pool has no subjects")
	}
	root3SubjectDN := root3Subjects[0]

	chainLeaf := parseLeaf(&idChain)
	if chainLeaf == nil {
		t.Fatal("parseLeaf returned nil for chain identity")
	}
	if string(chainLeaf.RawIssuer) == string(root3SubjectDN) {
		t.Fatal("test setup error: chain identity's leaf issuer should be the intermediate, not the root")
	}

	t.Run("matches via intermediate cert in chain", func(t *testing.T) {
		identities := []ClientIdentity{idAcme, idChain}
		got, err := selectIdentity(testCRI(root3SubjectDN), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "chain" {
			t.Fatalf("got %q, want chain", got.LoaderName)
		}
	})

	t.Run("chain identity at index 0", func(t *testing.T) {
		identities := []ClientIdentity{idChain, idAcme}
		got, err := selectIdentity(testCRI(root3SubjectDN), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "chain" {
			t.Fatalf("got %q, want chain", got.LoaderName)
		}
	})

	t.Run("direct-issue identity alongside chain identity", func(t *testing.T) {
		acmeIssuer := parseLeaf(&idAcme).RawIssuer
		identities := []ClientIdentity{idChain, idAcme}
		got, err := selectIdentity(testCRI(acmeIssuer), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "acme" {
			t.Fatalf("got %q, want acme", got.LoaderName)
		}
	})

	t.Run("no match falls back to first", func(t *testing.T) {
		identities := []ClientIdentity{idAcme, idChain}
		got, err := selectIdentity(testCRI([]byte("unknown-ca")), identities, logger)
		testutil.FatalOnErr("selectIdentity", err, t)
		if got.LoaderName != "acme" {
			t.Fatalf("got %q, want acme (fallback)", got.LoaderName)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMultiIdentityCredentials_InfoCloneOverride
// ---------------------------------------------------------------------------

func TestMultiIdentityCredentials_InfoCloneOverride(t *testing.T) {
	unregisterAll()
	registerTestLoaders(t, "simple", "acme")

	id := acmeIdentity(t)
	m, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "simple", testLogger(t), nil)
	testutil.FatalOnErr("new", err, t)

	info := m.Info()
	if info.SecurityProtocol != "tls" {
		t.Fatalf("SecurityProtocol = %q, want tls", info.SecurityProtocol)
	}
	if info.ServerName != "" { //nolint:staticcheck
		t.Fatalf("ServerName = %q, want empty", info.ServerName) //nolint:staticcheck
	}

	err = m.OverrideServerName("override-name") //nolint:staticcheck
	testutil.FatalOnErr("OverrideServerName", err, t)
	info = m.Info()
	if info.ServerName != "override-name" { //nolint:staticcheck
		t.Fatalf("ServerName = %q, want override-name", info.ServerName) //nolint:staticcheck
	}

	cloned := m.Clone().(*MultiIdentityCredentials)
	clonedInfo := cloned.Info()
	if clonedInfo.ServerName != "override-name" { //nolint:staticcheck
		t.Fatalf("Clone ServerName = %q, want override-name", clonedInfo.ServerName) //nolint:staticcheck
	}
	if len(cloned.identities) != 1 {
		t.Fatal("Clone didn't copy identities")
	}
	cloned.identities[0].LoaderName = "mutated"
	if m.identities[0].LoaderName == "mutated" {
		t.Fatal("Clone shares identity slice backing array")
	}
}

// ---------------------------------------------------------------------------
// TestMultiIdentityCredentials_ServerHandshake
// ---------------------------------------------------------------------------

func TestMultiIdentityCredentials_ServerHandshake(t *testing.T) {
	unregisterAll()
	registerTestLoaders(t, "simple", "acme")

	id := acmeIdentity(t)
	m, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "simple", testLogger(t), nil)
	testutil.FatalOnErr("new", err, t)
	_, _, err = m.ServerHandshake(nil)
	testutil.FatalOnNoErr("ServerHandshake should fail", err, t)
}

// ---------------------------------------------------------------------------
// TestLoadClientIdentity
// ---------------------------------------------------------------------------

func TestLoadClientIdentity(t *testing.T) {
	unregisterAll()
	err := Register("simple", &simpleLoader{name: "simple"})
	testutil.FatalOnErr("Register simple", err, t)
	err = Register("errorCert", &simpleLoader{name: "errorCert"})
	testutil.FatalOnErr("Register errorCert", err, t)

	ctx := context.Background()

	t.Run("good loader", func(t *testing.T) {
		id, err := LoadClientIdentity(ctx, "simple")
		testutil.FatalOnErr("LoadClientIdentity", err, t)
		if id.LoaderName != "simple" {
			t.Fatalf("LoaderName = %q, want simple", id.LoaderName)
		}
		if len(id.Cert.Certificate) == 0 {
			t.Fatal("Cert has no certificate chain")
		}
	})

	t.Run("unknown loader", func(t *testing.T) {
		_, err := LoadClientIdentity(ctx, "nonexistent")
		testutil.FatalOnNoErr("unknown loader", err, t)
	})

	t.Run("error loading cert", func(t *testing.T) {
		_, err := LoadClientIdentity(ctx, "errorCert")
		testutil.FatalOnNoErr("errorCert", err, t)
	})
}

// ---------------------------------------------------------------------------
// TestRefreshIfNeeded
// ---------------------------------------------------------------------------

type refreshLoader struct {
	refreshed bool
	loadErr   bool
}

func (r *refreshLoader) LoadClientCA(context.Context) (*x509.CertPool, error) {
	return LoadRootOfTrust("testdata/root.pem")
}
func (r *refreshLoader) LoadRootCA(context.Context) (*x509.CertPool, error) {
	return LoadRootOfTrust("testdata/root.pem")
}
func (r *refreshLoader) LoadServerCertificate(context.Context) (tls.Certificate, error) {
	return tls.LoadX509KeyPair("testdata/leaf.pem", "testdata/leaf.key")
}
func (r *refreshLoader) LoadClientCertificate(context.Context) (tls.Certificate, error) {
	if r.loadErr {
		return tls.Certificate{}, errLoadFailed
	}
	return tls.LoadX509KeyPair("testdata/client.pem", "testdata/client.key")
}
func (r *refreshLoader) CertsRefreshed() bool { return r.refreshed }
func (r *refreshLoader) GetClientCertInfo(context.Context, string) (*ClientCertInfo, error) {
	return nil, nil
}

var errLoadFailed = x509.UnknownAuthorityError{}

func TestRefreshIfNeeded(t *testing.T) {
	unregisterAll()
	rl := &refreshLoader{}
	err := Register("refreshable", rl)
	testutil.FatalOnErr("Register", err, t)

	id, err := LoadClientIdentity(context.Background(), "refreshable")
	testutil.FatalOnErr("LoadClientIdentity", err, t)
	m, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "refreshable", testLogger(t), nil)
	testutil.FatalOnErr("new", err, t)

	t.Run("no refresh needed", func(t *testing.T) {
		rl.refreshed = false
		err := m.refreshIfNeeded(context.Background())
		testutil.FatalOnErr("refreshIfNeeded", err, t)
	})

	t.Run("refresh succeeds", func(t *testing.T) {
		rl.refreshed = true
		rl.loadErr = false
		err := m.refreshIfNeeded(context.Background())
		testutil.FatalOnErr("refreshIfNeeded", err, t)
	})

	t.Run("refresh fails returns error", func(t *testing.T) {
		rl.refreshed = true
		rl.loadErr = true
		err := m.refreshIfNeeded(context.Background())
		testutil.FatalOnNoErr("refreshIfNeeded with load error", err, t)
	})
}

// ---------------------------------------------------------------------------
// TestClientHandshake_E2E — full TLS handshake with identity selection
// ---------------------------------------------------------------------------

func TestClientHandshake_SingleIdentity(t *testing.T) {
	unregisterAll()
	registerTestLoaders(t, "simple")

	serverCert, err := tls.LoadX509KeyPair("testdata/leaf.pem", "testdata/leaf.key")
	testutil.FatalOnErr("server keypair", err, t)
	clientCA, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("client CA", err, t)

	lis := tlsServer(t, serverCert, clientCA)

	id := loadClientIdentity(t, "testdata/client.pem", "testdata/client.key", "simple")
	m, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "simple", testLogger(t), nil)
	testutil.FatalOnErr("new", err, t)

	rawConn, err := net.Dial("tcp", lis.Addr().String())
	testutil.FatalOnErr("Dial", err, t)
	defer rawConn.Close()

	conn, info, err := m.ClientHandshake(context.Background(), "localhost", rawConn)
	testutil.FatalOnErr("ClientHandshake", err, t)
	defer conn.Close()
	if info == nil {
		t.Fatal("AuthInfo is nil")
	}
}

func TestClientHandshake_SelectsCorrectIdentity(t *testing.T) {
	unregisterAll()
	registerTestLoaders(t, "acme", "other")
	testutil.FatalOnErr("Register merged", Register("merged", &mergedRootLoader{}), t)

	serverCert1, err := tls.LoadX509KeyPair("testdata/leaf.pem", "testdata/leaf.key")
	testutil.FatalOnErr("server1 keypair", err, t)
	clientCA1, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("client CA1", err, t)

	serverCert2, err := tls.LoadX509KeyPair("testdata/leaf2.pem", "testdata/leaf2.key")
	testutil.FatalOnErr("server2 keypair", err, t)
	clientCA2, err := LoadRootOfTrust("testdata/root2.pem")
	testutil.FatalOnErr("client CA2", err, t)

	idAcme := loadClientIdentity(t, "testdata/client.pem", "testdata/client.key", "acme")
	idOther := loadClientIdentity(t, "testdata/client2.pem", "testdata/client2.key", "other")

	m, err := NewMultiIdentityCredentials([]ClientIdentity{idAcme, idOther}, "merged", testLogger(t), nil)
	testutil.FatalOnErr("new", err, t)

	t.Run("server trusting Acme gets acme cert", func(t *testing.T) {
		lis := tlsServer(t, serverCert1, clientCA1)
		rawConn, err := net.Dial("tcp", lis.Addr().String())
		testutil.FatalOnErr("Dial", err, t)
		defer rawConn.Close()

		conn, _, err := m.ClientHandshake(context.Background(), "localhost", rawConn)
		testutil.FatalOnErr("ClientHandshake to acme server", err, t)
		conn.Close()
	})

	t.Run("server trusting Other gets other cert", func(t *testing.T) {
		lis := tlsServer(t, serverCert2, clientCA2)
		rawConn, err := net.Dial("tcp", lis.Addr().String())
		testutil.FatalOnErr("Dial", err, t)
		defer rawConn.Close()

		conn, _, err := m.ClientHandshake(context.Background(), "localhost", rawConn)
		testutil.FatalOnErr("ClientHandshake to other server", err, t)
		conn.Close()
	})
}

func TestClientHandshake_IntermediateCA(t *testing.T) {
	unregisterAll()
	registerTestLoaders(t, "acme", "chain")
	testutil.FatalOnErr("Register merged", Register("merged", &mergedRootLoader{}), t)

	serverCert, err := tls.LoadX509KeyPair("testdata/leaf3_chain.pem", "testdata/leaf3.key")
	testutil.FatalOnErr("server3 keypair", err, t)
	clientCA, err := LoadRootOfTrust("testdata/root3.pem")
	testutil.FatalOnErr("client CA3", err, t)

	lis := tlsServer(t, serverCert, clientCA)

	idAcme := loadClientIdentity(t, "testdata/client.pem", "testdata/client.key", "acme")
	idChain := loadClientIdentity(t, "testdata/client3.pem", "testdata/client3.key", "chain")

	m, err := NewMultiIdentityCredentials([]ClientIdentity{idAcme, idChain}, "merged", testLogger(t), nil)
	testutil.FatalOnErr("new", err, t)

	rawConn, err := net.Dial("tcp", lis.Addr().String())
	testutil.FatalOnErr("Dial", err, t)
	defer rawConn.Close()

	conn, info, err := m.ClientHandshake(context.Background(), "localhost", rawConn)
	testutil.FatalOnErr("ClientHandshake with intermediate CA chain", err, t)
	defer conn.Close()
	if info == nil {
		t.Fatal("AuthInfo is nil")
	}
}

// TestClientHandshake_WrongCertFails verifies that when the only available
// identity is not trusted by the server, the handshake fails on at least
// one side (client or server). This exercises the error/metrics path in
// ClientHandshake.
func TestClientHandshake_WrongCertFails(t *testing.T) {
	unregisterAll()
	registerTestLoaders(t, "acme")
	testutil.FatalOnErr("Register merged", Register("merged", &mergedRootLoader{}), t)

	serverCert, err := tls.LoadX509KeyPair("testdata/leaf2.pem", "testdata/leaf2.key")
	testutil.FatalOnErr("server keypair", err, t)
	clientCA, err := LoadRootOfTrust("testdata/root2.pem")
	testutil.FatalOnErr("client CA", err, t)

	cfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    clientCA,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2"},
	}
	lis, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	testutil.FatalOnErr("tls.Listen", err, t)
	t.Cleanup(func() { lis.Close() })

	serverErr := make(chan error, 1)
	go func() {
		conn, err := lis.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		if tlsConn, ok := conn.(*tls.Conn); ok {
			serverErr <- tlsConn.Handshake()
		} else {
			serverErr <- nil
		}
	}()

	id := loadClientIdentity(t, "testdata/client.pem", "testdata/client.key", "acme")
	m, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "merged", testLogger(t), nil)
	testutil.FatalOnErr("new", err, t)

	rawConn, err := net.Dial("tcp", lis.Addr().String())
	testutil.FatalOnErr("Dial", err, t)
	defer rawConn.Close()

	_, _, clientErr := m.ClientHandshake(context.Background(), "localhost", rawConn)

	srvErr := <-serverErr
	if clientErr == nil && srvErr == nil {
		t.Fatal("expected either client or server handshake to fail (wrong client cert CA), but both succeeded")
	}
}

// ---------------------------------------------------------------------------
// TestConcurrency — run multiple operations in parallel under -race
// ---------------------------------------------------------------------------

func TestConcurrency(t *testing.T) {
	unregisterAll()
	registerTestLoaders(t, "simple")

	serverCert, err := tls.LoadX509KeyPair("testdata/leaf.pem", "testdata/leaf.key")
	testutil.FatalOnErr("server keypair", err, t)
	clientCA, err := LoadRootOfTrust("testdata/root.pem")
	testutil.FatalOnErr("client CA", err, t)

	lis := tlsServer(t, serverCert, clientCA)

	id := loadClientIdentity(t, "testdata/client.pem", "testdata/client.key", "simple")
	m, err := NewMultiIdentityCredentials([]ClientIdentity{id}, "simple", testLogger(t), nil)
	testutil.FatalOnErr("new", err, t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Info()
			_ = m.Clone()

			rawConn, err := net.Dial("tcp", lis.Addr().String())
			if err != nil {
				return
			}
			conn, _, err := m.ClientHandshake(context.Background(), "localhost", rawConn)
			if err != nil {
				rawConn.Close()
				return
			}
			conn.Close()
		}()
	}
	wg.Wait()
}
