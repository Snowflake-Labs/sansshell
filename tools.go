//go:build tools
// +build tools

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

package sanshell

// Various tooling uses by the sansshell build process.
// These import lines make tools (such as code generators)
// visible to Go's module tracking, ensuring consistent
// versioning in multiple environments.
import (
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)

//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go
//go:generate go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

// For convenience, listing of service generation
// targets makes it possible to regenerate all services
// by executing `go generate` against this file.

//go:generate go generate ./proxy
//go:generate go generate ./proxy/testdata
//go:generate go generate ./services/ansible
//go:generate go generate ./services/dns
//go:generate go generate ./services/exec
//go:generate go generate ./services/fdb
//go:generate go generate ./services/healthcheck
//go:generate go generate ./services/localfile
//go:generate go generate ./services/network
//go:generate go generate ./services/packages
//go:generate go generate ./services/process
//go:generate go generate ./services/sansshell
//go:generate go generate ./services/service
//go:generate go generate ./services/sysinfo
