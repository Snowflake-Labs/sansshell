package testdata

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative --go-grpcproxy_out=. --go-grpcproxy_opt=paths=source_relative testservice.proto
