package services

import (
	"sync"

	"google.golang.org/grpc"
)

var (
	mu          sync.RWMutex
	rpcServices []UnshelledRpcService
)

type UnshelledRpcService interface {
	Register(*grpc.Server)
}

// RegisterUnshelledService provides a mechanism for imported modules to
// register themselves with a gRPC server.
func RegisterUnshelledService(s UnshelledRpcService) {
	mu.Lock()
	defer mu.Unlock()
	rpcServices = append(rpcServices, s)
}

// ListServices returns the list of registered serfvices.
func ListServices() []UnshelledRpcService {
	mu.RLock()
	defer mu.RUnlock()
	return rpcServices
}
