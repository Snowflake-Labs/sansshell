package server

import (
	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/service"
)

type sansshellServer struct {
	server pb.ServiceServer
}

// See: services.SansShellRpcService
func (s *sansshellServer) Register(gs *grpc.Server) {
	pb.RegisterServiceServer(gs, s.server)
}

func init() {
	services.RegisterSansShellService(&sansshellServer{
		server: createServer(),
	})
}
