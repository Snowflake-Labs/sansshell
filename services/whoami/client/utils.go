/*
Copyright (c) 2025 Snowflake Inc. All rights reserved.

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
package client

import (
	"crypto/x509"
	"fmt"
	"time"
)

func roundRemainingTime(t time.Time) (int, int, int) {
	remaining := time.Until(t).Round(time.Minute)
	days := int(remaining.Hours()) / 24
	hours := int(remaining.Hours()) % 24
	minutes := int(remaining.Minutes()) % 60
	return days, hours, minutes
}

func ShowClientInfoFromCert(x509Cert *x509.Certificate) {
	fmt.Println("username:", x509Cert.Subject.CommonName)
	fmt.Println("cert:")
	fmt.Println("\tissuer:", x509Cert.Issuer.CommonName)
	days, hours, minutes := roundRemainingTime(x509Cert.NotAfter)
	if days < 0 || hours < 0 || minutes < 0 {
		fmt.Printf("\tvalid until: %v (expired)\n", x509Cert.NotAfter)
	} else {
		fmt.Printf("\tvalid until: %v (%d days %d hours %d minutes)\n", x509Cert.NotAfter, days, hours, minutes)
	}
}

func showGroupsFromCert(x509Cert *x509.Certificate) {
	fmt.Println("groups:")
	for _, group := range x509Cert.Subject.OrganizationalUnit {
		fmt.Println("\t", group)
	}
}
