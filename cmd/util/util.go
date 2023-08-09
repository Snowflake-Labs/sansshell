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
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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

// hasPort returns true if the provided address does not include a port number.
func hasPort(s string) bool {
	return strings.LastIndex(s, "]") < strings.LastIndex(s, ":")
}

// swappable for testing
var logFatalf = log.Fatalf

// ValidateAndAddPort will take a given target address and optionally add a default port
// onto it if a port is missing. This will also take in account optional dial timeout
// suffix and make sure the returned value has that if it exists.
func ValidateAndAddPort(s string, port int) string {
	return ValidateAndAddPortAndTimeout(s, port, 0)
}

// ValidateAndAddPortAndTimeout will take a given target address and optionally add a default port and timeout
// onto it if a port or a timeout is missing.
func ValidateAndAddPortAndTimeout(s string, port int, dialTimeout time.Duration) string {
	// See if there's a duration appended and pull it off
	p := strings.Split(s, ";")
	if len(p) == 0 || len(p) > 2 || p[0] == "" {
		logFatalf("Invalid address %q - should be of the form host[:port][;<duration>]", s)
	}
	new := s
	if !hasPort(p[0]) {
		new = fmt.Sprintf("%s:%d", p[0], port)
		if len(p) == 2 {
			// Add duration back if we pulled it off.
			new = fmt.Sprintf("%s;%s", new, p[1])
		}
	}
	if len(p) == 2 { // timeout exists, validate it
		timeout := p[1]
		if _, err := time.ParseDuration(timeout); err != nil {
			logFatalf("Invalid timeout %s - should be of the form time.Duration", timeout)
		}
	} else { // no timeout, let's add dialTimeout
		if dialTimeout != 0 {
			new = fmt.Sprintf("%s;%s", new, dialTimeout.String())
		}
	}
	return new
}
