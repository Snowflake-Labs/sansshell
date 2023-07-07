//go:build !linux
// +build !linux

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
	// YumBin is the location of the yum binary. On non-linux this is blank as it's likely unsupported.
	YumBin string
	// YumCleanup is the location of the yum-complete-transaction binary. On non-linux this is blank as it's likely unsupported.
	YumCleanup string
	//NeedsRestartingBin is the location of needs-restarting tool. Supported only on linux.
	NeedsRestartingBin string
	// RepoqueryBin is the location of the repoquery binary. On non-linux this is blank as it's likely unsupported.
	RepoqueryBin string
)
