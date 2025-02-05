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
	"fmt"

	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/credentials"
)

// LoadServerCredentials returns transport credentials for a SansShell server as
// retrieved from the specified `loaderName`. This should be the most commonly
// used method to generate credentials as this will support reloadable credentials
// as the TransportCredentials returned are a WrappedTransportCredentials which
// will check at call time if new certificates are available.
func LoadServerCredentials(ctx context.Context, loaderName string) (credentials.TransportCredentials, error) {
	wrapped, _, err := LoadServerCredentialsWithForceRefresh(ctx, loaderName)
	return wrapped, err
}

// LoadServerCredentialsWithForceRefresh returns transport credentials along with
// a function that allows immediately refreshing the credentials
func LoadServerCredentialsWithForceRefresh(ctx context.Context, loaderName string) (credentials.TransportCredentials, func() error, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	mtlsLoader, err := Loader(loaderName)
	if err != nil {
		return nil, nil, err
	}
	creds, err := internalLoadServerCredentials(ctx, loaderName)
	if err != nil {
		return nil, nil, err
	}
	wrapped := &WrappedTransportCredentials{
		creds:      creds,
		loaderName: loaderName,
		loader:     internalLoadServerCredentials,
		mtlsLoader: mtlsLoader,
		logger:     logger,
		recorder:   recorder,
	}
	return wrapped, wrapped.refreshNow, err
}

func internalLoadServerCredentials(ctx context.Context, loaderName string) (credentials.TransportCredentials, error) {
	logger := logr.FromContextOrDiscard(ctx)
	loader, err := Loader(loaderName)
	if err != nil {
		return nil, err
	}

	pool, err := loader.LoadClientCA(ctx)
	if err != nil {
		return nil, err
	}
	logger.Info("loading new server cert")
	cert, err := loader.LoadServerCertificate(ctx)
	if err != nil {
		return nil, err
	}
	logger.Info("loaded new server cert", "error", err)
	return NewServerCredentials(cert, pool), nil
}

// NewServerCredentials creates transport credentials for a SansShell server.
// NOTE: This doesn't support reloadable credentials.
func NewServerCredentials(cert tls.Certificate, CAPool *x509.CertPool) credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    CAPool,
		MinVersion:   tls.VersionTLS12,
	})
}

// LoadServerTLS reads the certificates and keys from disk at the supplied paths,
// and assembles them into a set of TransportCredentials for the gRPC server.
// NOTE: This doesn't support reloadable credentials.
func LoadServerTLS(clientCertFile, clientKeyFile string, CAPool *x509.CertPool) (credentials.TransportCredentials, error) {
	// Read in client credentials
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("reading client credentials: %w", err)
	}
	return NewServerCredentials(cert, CAPool), nil
}
