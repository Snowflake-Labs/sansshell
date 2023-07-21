//go:build linux
// +build linux

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

package server

var (
	// YumBin is the location of the yum binary. Binding this to a flag is often useful.
	YumBin = "/usr/bin/yum"
	// YumCleanup is the location of the yum-complete-transaction binary. Binding this to a flag is often useful.
	YumCleanup = "/sbin/yum-complete-transaction"
	// NeedsRestartingBin is location of the needs-restarting binary.
	NeedsRestartingBin = "/bin/needs-restarting"
	// RepoqueryBin is the location of the repoquery binary. Binding this to a flag is often useful.
	RepoqueryBin = "/usr/bin/repoquery"
)
