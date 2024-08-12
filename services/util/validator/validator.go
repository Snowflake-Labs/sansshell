/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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
)

func IsValidPort(val int) error {
	if val < 1 || val > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	return nil
}

func IsValidPortUint32(val uint32) error {
	if val < 1 || val > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	return nil
}
