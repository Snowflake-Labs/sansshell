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
	"path/filepath"
)

// LoadRootOfTrust will load an CA root of trust(s) from the given
// file and return a CertPool to use in validating certificates.
// All CA's to validate against must be presented together in the PEM
// file.
// If the file is a directory, LoadRootOfTrust will load all files
// in that directory.
func LoadRootOfTrust(path string) (*x509.CertPool, error) {
	capool := x509.NewCertPool()

	loadRoot := func(filename string) error {
		ca, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("could not read %q: %w", filename, err)
		}
		if !capool.AppendCertsFromPEM(ca) {
			return fmt.Errorf("could not add CA cert from %q to pool: %w", filename, err)
		}
		return nil
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("could not stat CA cert path %q: %w", path, err)
	}
	if fi.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("could not read  CA cert directory %q: %w", path, err)
		}
		for _, f := range files {
			if err := loadRoot(filepath.Join(path, f.Name())); err != nil {
				return nil, err
			}
		}
	} else {
		err := loadRoot(path)
		if err != nil {
			return nil, err
		}
	}
	return capool, nil
}
