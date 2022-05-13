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

// Package flags provides flag support for loading client/server certs and CA root of trust.
package flags

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"path"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
)

const (
	loaderName = "flags"
)

var (
	// ClientCertFile is the location to load the client cert. Binding this to a flag is often useful.
	ClientCertFile = path.Join(os.Getenv("HOME"), ".sansshell/client.pem")

	// ClientKeyFile is the location to load the client cert key. Binding this to a flag is often useful.
	ClientKeyFile = path.Join(os.Getenv("HOME"), ".sansshell/client.key")

	// ServerCertFile is the location to load the server cert. Binding this to a flag is often useful.
	ServerCertFile = path.Join(os.Getenv("HOME"), ".sansshell/leaf.pem")

	// ServerKeyFile is the location to load the server cert key. Binding this to a flag is often useful.
	ServerKeyFile = path.Join(os.Getenv("HOME"), ".sansshell/leaf.key")

	// RootCAFile is the location to load the root CA store. Binding this to a flag is often useful.
	RootCAFile = path.Join(os.Getenv("HOME"), ".sansshell/root.pem")
)

// Name returns the loader to use to set mtls params via flags.
func Name() string { return loaderName }

// flagLoader implements mtls.CredentialsLoader by reading files specified
// by command-line flags.
type flagLoader struct{}

func (flagLoader) LoadClientCA(context.Context) (*x509.CertPool, error) {
	return mtls.LoadRootOfTrust(RootCAFile)
}

func (flagLoader) LoadRootCA(context.Context) (*x509.CertPool, error) {
	return mtls.LoadRootOfTrust(RootCAFile)
}

func (flagLoader) LoadClientCertificate(context.Context) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(ClientCertFile, ClientKeyFile)
}

func (flagLoader) LoadServerCertificate(context.Context) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(ServerCertFile, ServerKeyFile)
}

func (flagLoader) CertsRefreshed() bool {
	return false
}

func init() {
	if err := mtls.Register(loaderName, flagLoader{}); err != nil {
		panic(err)
	}
}
