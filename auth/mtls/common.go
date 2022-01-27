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
	"crypto/x509"
	"fmt"
	"os"
)

// LoadRootOfTrust will load an CA root of trust from the given
// file and return a CertPool to use in validating certificates.
func LoadRootOfTrust(filename string) (*x509.CertPool, error) {
	// Read in the root of trust for client identities
	ca, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("could not read CA from %q: %w", filename, err)
	}
	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("could not add CA cert to pool: %w", err)
	}
	return capool, nil
}
