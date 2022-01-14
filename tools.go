//go:build tools
// +build tools

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

//go:generate go generate ./proxy/testdata
//go:generate go generate ./services/ansible
//go:generate go generate ./services/exec
//go:generate go generate ./services/healthcheck
//go:generate go generate ./services/localfile
//go:generate go generate ./services/packages
//go:generate go generate ./services/process
//go:generate go generate ./services/service
