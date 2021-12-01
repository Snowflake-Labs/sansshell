package server

import (
	"context"
	"log"
	"net"
	"os"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
	conn    *grpc.ClientConn
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	lfs := &server{}
	lfs.Register(s)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

func TestRead(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewHealthCheckClient(conn)
	_, err = client.Ok(ctx, &pb.Empty{})
	testutil.FatalOnErr("HealthCheck failed", err, t)
}
