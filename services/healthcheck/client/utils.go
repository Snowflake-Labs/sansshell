package client

import (
	"context"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"google.golang.org/protobuf/types/known/emptypb"
)

func HealthcheckValidateMany(ctx context.Context, conn *proxy.Conn) ([]*pb.OkManyResponse, error) {
	c := pb.NewHealthCheckClientProxy(conn)

	respChan, err := c.OkOneMany(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	results := make([]*pb.OkManyResponse, len(conn.Targets))
	for r := range respChan {
		results[r.Index] = r
	}

	return results, nil
}
