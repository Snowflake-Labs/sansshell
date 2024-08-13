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

package cli

import (
	writerUtils "github.com/Snowflake-Labs/sansshell/services/util/writer"
	"golang.org/x/term"
	"io"
	"os"
)

func IsStreamToTerminal(stream io.Writer) bool {
	switch v := stream.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	case writerUtils.WrappedWriter:
		return IsStreamToTerminal(v.GetOriginal())
	default:
		return false
	}
}
