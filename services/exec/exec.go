package exec

// To regenerate the proto headers if the .proto changes, just run go generate
// This comment encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative exec.proto
