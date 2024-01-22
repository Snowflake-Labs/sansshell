/* Copyright (c) 2024 Snowflake Inc. All rights reserved.

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

package network

// To regenerate the proto headers if the proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative --go-grpcproxy_out=. --go-grpcproxy_opt=paths=source_relative network.proto

// YYYY-MM-DD HH:MM:SS time format
const TimeFormat_YYYYMMDDHHMMSS = "2006-01-02 15:04:05"

// Maximum journal entries we can fetch for each host
const JounalEntriesLimit = 10000
