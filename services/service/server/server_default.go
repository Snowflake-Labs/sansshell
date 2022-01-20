//go:build !linux
// +build !linux

package server

import (
	pb "github.com/Snowflake-Labs/sansshell/services/service"
)

func createServer() pb.ServiceServer {
	return pb.UnimplementedServiceServer{}
}
