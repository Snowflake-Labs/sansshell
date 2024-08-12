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
	"fmt"
	"io"
)

var restoreFormatingCode = "\033[0m"
var RED_TEXT_COLOR = "\033[31m"
var GREEN_TEXT_COLOT = "\033[32m"
var YELLOW_TEXT_COLOR = "\033[33m"

func ColoredFprintf(w io.Writer, color string, format string, a ...any) (n int, err error) {
	line := fmt.Sprintf(format, a...)
	return fmt.Print(color, line, restoreFormatingCode)
}

func ColoredSprintf(color string, format string, a ...any) string {
	line := fmt.Sprintf(format, a...)
	return fmt.Sprint(color, line, restoreFormatingCode)
}

func ColoredSprint(color string, a ...any) string {
	line := fmt.Sprint(a...)
	return fmt.Sprint(color, line, restoreFormatingCode)
}
