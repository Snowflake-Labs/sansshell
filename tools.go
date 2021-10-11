// +build tools

package sanshell

// tools used by unshelled build process
// These import lines make tools (such as code generators)
// visible to Go's module tracking, ensuring consistent
// versioning.
import (
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)

// As a convenience, the various
//go:generate go generate ./services/exec
//go:generate go generate ./services/healthcheck
//go:generate go generate ./services/localfile
//go:generate go generate ./services/packages
