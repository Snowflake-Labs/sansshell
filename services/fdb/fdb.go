// Package fdb defines the RPC interface for the sansshell FDB actions.
package fdb

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative --go-grpcproxy_out=. --go-grpcproxy_opt=paths=source_relative fdb.proto
