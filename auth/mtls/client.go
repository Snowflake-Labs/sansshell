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

// LoadClientCredentials returns transport credentials for SansShell clients,
// based on the provided `loaderName`
func LoadClientCredentials(ctx context.Context, loaderName string) (credentials.TransportCredentials, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	mtlsLoader, err := Loader(loaderName)
	if err != nil {
		return nil, err
	}
	creds, err := internalLoadClientCredentials(ctx, loaderName)
	if err != nil {
		return nil, err
	}
	wrapped := &WrappedTransportCredentials{
		creds:      creds,
		loaderName: loaderName,
		loader:     internalLoadClientCredentials,
		mtlsLoader: mtlsLoader,
		logger:     logger,
		recorder:   recorder,
	}
	return wrapped, nil
}

func internalLoadClientCredentials(ctx context.Context, loaderName string) (credentials.TransportCredentials, error) {
	logger := logr.FromContextOrDiscard(ctx)
	loader, err := Loader(loaderName)
	if err != nil {
		return nil, err
	}

	pool, err := loader.LoadRootCA(ctx)
	if err != nil {
		return nil, err
	}
	logger.Info("loading new client cert")
	cert, err := loader.LoadClientCertificate(ctx)
	if err != nil {
		return nil, err
	}
	logger.Info("loaded new client cert", "error", err)
	return NewClientCredentials(cert, pool), nil
}

// NewClientCredentials returns transport credentials for SansShell clients.
func NewClientCredentials(cert tls.Certificate, CAPool *x509.CertPool) credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            CAPool,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
	})
}

// LoadClientTLS reads the certificates and keys from disk at the supplied paths,
// and assembles them into a set of TransportCredentials for the gRPC client.
func LoadClientTLS(clientCertFile, clientKeyFile string, CAPool *x509.CertPool) (credentials.TransportCredentials, error) {
	// Read in client credentials
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read client credentials: %w", err)
	}
	return NewClientCredentials(cert, CAPool), nil
}
