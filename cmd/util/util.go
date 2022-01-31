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

// Package util provides functions used across command line binaries for setup/exection.
package util

import (
	"errors"
	"os"

	"github.com/go-logr/logr"
)

// ChoosePolicy selects an OPA policy based on the flags, or calls log.Fatal if
// an invalid combination is provided.
func ChoosePolicy(logger logr.Logger, defaultPolicy string, policyFlag string, policyFile string) string {
	if policyFlag != defaultPolicy && policyFile != "" {
		logger.Error(errors.New("invalid policy flags"), "do not set both --policy and --policy-file")
		os.Exit(1)
	}

	var policy string
	if policyFile != "" {
		pff, err := os.ReadFile(policyFile)
		if err != nil {
			logger.Error(err, "os.ReadFile(policyFile)", "file", policyFile)
		}
		logger.Info("using policy from --policy-file", "file", policyFile)
		policy = string(pff)
	} else {
		if policyFlag != defaultPolicy {
			logger.Info("using policy from --policy")
			policy = policyFlag
		} else {
			logger.Info("using built-in policy")
			policy = defaultPolicy
		}
	}
	return policy
}
