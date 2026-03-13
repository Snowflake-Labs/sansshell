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

package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	proxyserver "github.com/Snowflake-Labs/sansshell/proxy/server"
)

const (
	failLoaderName = "test-buildDialers-fail"
	okLoaderName   = "test-buildDialers-ok"
)

func TestMain(m *testing.M) {
	if err := mtls.Register(failLoaderName, failingLoader{}); err != nil {
		fmt.Fprintf(os.Stderr, "mtls.Register(%s): %v\n", failLoaderName, err)
		os.Exit(1)
	}
	if err := mtls.Register(okLoaderName, successLoader{}); err != nil {
		fmt.Fprintf(os.Stderr, "mtls.Register(%s): %v\n", okLoaderName, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

type fakeDialer struct{}

func (fakeDialer) DialContext(_ context.Context, _ string, _ ...grpc.DialOption) (proxyserver.ClientConnCloser, error) {
	return nil, errors.New("not implemented")
}

func TestWithNamedClientCredSourceSingle(t *testing.T) {
	rs := &runState{}
	opt := WithNamedClientCredSource("pg", "some-loader")
	if err := opt.apply(context.Background(), rs); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, ok := rs.namedCredSources["pg"]; !ok || got != "some-loader" {
		t.Fatalf("expected namedCredSources[\"pg\"] = \"some-loader\", got %q (ok=%v)", got, ok)
	}
}

func TestWithNamedClientCredSourceMultiple(t *testing.T) {
	rs := &runState{}
	for _, pair := range []struct{ hint, src string }{
		{"pg", "loader-a"},
		{"redis", "loader-b"},
	} {
		if err := WithNamedClientCredSource(pair.hint, pair.src).apply(context.Background(), rs); err != nil {
			t.Fatalf("apply(%q): %v", pair.hint, err)
		}
	}
	if len(rs.namedCredSources) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(rs.namedCredSources))
	}
	if rs.namedCredSources["pg"] != "loader-a" {
		t.Fatalf("pg: got %q", rs.namedCredSources["pg"])
	}
	if rs.namedCredSources["redis"] != "loader-b" {
		t.Fatalf("redis: got %q", rs.namedCredSources["redis"])
	}
}

func TestWithNamedClientCredSourceOverwrite(t *testing.T) {
	rs := &runState{}
	if err := WithNamedClientCredSource("pg", "old").apply(context.Background(), rs); err != nil {
		t.Fatal(err)
	}
	if err := WithNamedClientCredSource("pg", "new").apply(context.Background(), rs); err != nil {
		t.Fatal(err)
	}
	if rs.namedCredSources["pg"] != "new" {
		t.Fatalf("expected overwrite to \"new\", got %q", rs.namedCredSources["pg"])
	}
}

// --- buildDialers tests ---

type failingLoader struct{}

func (failingLoader) LoadClientCA(context.Context) (*x509.CertPool, error) {
	return nil, errors.New("no CA")
}
func (failingLoader) LoadRootCA(context.Context) (*x509.CertPool, error) {
	return nil, errors.New("no root CA")
}
func (failingLoader) LoadClientCertificate(context.Context) (tls.Certificate, error) {
	return tls.Certificate{}, errors.New("no client cert")
}
func (failingLoader) LoadServerCertificate(context.Context) (tls.Certificate, error) {
	return tls.Certificate{}, errors.New("no server cert")
}
func (failingLoader) CertsRefreshed() bool { return false }
func (failingLoader) GetClientCertInfo(context.Context, string) (*mtls.ClientCertInfo, error) {
	return nil, nil
}

type successLoader struct{}

func (successLoader) LoadClientCA(context.Context) (*x509.CertPool, error) {
	return x509.NewCertPool(), nil
}
func (successLoader) LoadRootCA(context.Context) (*x509.CertPool, error) {
	return x509.NewCertPool(), nil
}
func (successLoader) LoadClientCertificate(context.Context) (tls.Certificate, error) {
	return tls.Certificate{}, nil
}
func (successLoader) LoadServerCertificate(context.Context) (tls.Certificate, error) {
	return tls.Certificate{}, nil
}
func (successLoader) CertsRefreshed() bool { return false }
func (successLoader) GetClientCertInfo(context.Context, string) (*mtls.ClientCertInfo, error) {
	return nil, nil
}

func TestBuildDialersDefaultOnly(t *testing.T) {
	rs := &runState{logger: logr.Discard()}
	dialers := buildDialers(context.Background(), rs, fakeDialer{}, nil)
	if _, ok := dialers[""]; !ok {
		t.Fatal("expected default dialer under key \"\"")
	}
	if len(dialers) != 1 {
		t.Fatalf("expected 1 dialer, got %d", len(dialers))
	}
}

func TestBuildDialersSkipsFailedCredSources(t *testing.T) {
	rs := &runState{
		logger:           logr.Discard(),
		namedCredSources: map[string]string{"bad-hint": failLoaderName},
	}
	dialers := buildDialers(context.Background(), rs, fakeDialer{}, nil)

	if _, ok := dialers[""]; !ok {
		t.Fatal("expected default dialer under key \"\"")
	}
	if _, ok := dialers["bad-hint"]; ok {
		t.Fatal("expected failed hint to be absent from dialers map")
	}
	if len(dialers) != 1 {
		t.Fatalf("expected 1 dialer (default only), got %d", len(dialers))
	}
}

func TestBuildDialersRegistersSuccessfulHint(t *testing.T) {
	rs := &runState{
		logger:           logr.Discard(),
		namedCredSources: map[string]string{"good-hint": okLoaderName},
	}
	shared := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4 * 1024 * 1024)),
	}
	dialers := buildDialers(context.Background(), rs, fakeDialer{}, shared)

	if _, ok := dialers[""]; !ok {
		t.Fatal("expected default dialer under key \"\"")
	}
	if _, ok := dialers["good-hint"]; !ok {
		t.Fatal("expected successful hint to be present in dialers map")
	}
	if len(dialers) != 2 {
		t.Fatalf("expected 2 dialers, got %d", len(dialers))
	}
}
