/* Copyright (c) 2025 Snowflake Inc. All rights reserved.

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
	"fmt"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
)

func roundRemainingTime(t time.Time) (int, int, int) {
	remaining := time.Until(t).Round(time.Minute)
	days := int(remaining.Hours()) / 24
	hours := int(remaining.Hours()) % 24
	minutes := int(remaining.Minutes()) % 60
	return days, hours, minutes
}

func showClientInfoFromClientCertInfo(certInfo mtls.ClientCertInfo) {
	fmt.Println("username:", certInfo.Username)
	fmt.Println("cert:")
	fmt.Println("\tissuer:", certInfo.CertIssuer)
	days, hours, minutes := roundRemainingTime(certInfo.ValidUntil)
	if days < 0 || hours < 0 || minutes < 0 {
		fmt.Printf("\tvalid until: %v (expired)\n", certInfo.ValidUntil)
	} else {
		fmt.Printf("\tvalid until: %v (%d days %d hours %d minutes)\n", certInfo.ValidUntil, days, hours, minutes)
	}
	fmt.Println("groups:")
	for _, group := range certInfo.Groups {
		fmt.Println("\t", group)
	}
}
