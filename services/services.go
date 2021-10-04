package services

import (
	"sync"

	"google.golang.org/grpc"
)

var (
	mu          sync.RWMutex
	rpcServices []SansShellRpcService
)

type SansShellRpcService interface {
	Register(*grpc.Server)
}

// RegisterSansShellService provides a mechanism for imported modules to
// register themselves with a gRPC server.
func RegisterSansShellService(s SansShellRpcService) {
	mu.Lock()
	defer mu.Unlock()
	rpcServices = append(rpcServices, s)
}

// ListServices returns the list of registered serfvices.
func ListServices() []SansShellRpcService {
	mu.RLock()
	defer mu.RUnlock()
	return rpcServices
}
