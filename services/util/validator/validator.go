/*
Copyright (c) 2019 Snowflake Inc. All rights reserved.

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

package validator

import (
	"errors"
	"strconv"
	"strings"
)

func ParsePortFromInt(val int) (uint32, error) {
	if val < 1 || val > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}

	return uint32(val), nil
}

func ParsePortFromUint32(val uint32) (uint32, error) {
	if val < 1 || val > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}

	return val, nil
}

func ParseHostAndPort(hostAndPort string) (string, uint32, error) {
	artifacts := strings.Split(hostAndPort, ":")
	if len(artifacts) != 2 {
		return "", 0, errors.New("invalid host:port format")
	}

	rawHost := strings.TrimSpace(artifacts[0])
	rawPort := artifacts[1]

	if rawHost == "" {
		return "", 0, errors.New("host cannot be empty")
	}
	host := rawHost

	intPort, err := strconv.Atoi(rawPort)
	if err != nil {
		return "", 0, errors.New("port must be a number")
	}

	port, err := ParsePortFromInt(intPort)
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}
