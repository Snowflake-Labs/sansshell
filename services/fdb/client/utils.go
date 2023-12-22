package client

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
)

func FDBCLIUtils(ctx context.Context, conn *proxy.Conn, style string) ([]*pb.FDBCLIManyResponse, error) {
	req := &pb.FDBCLIRequest{
		Commands: []*pb.FDBCLICommand{
			{
				Command: &pb.FDBCLICommand_Status{
					Status: &pb.FDBCLIStatus{
						Style: &wrapperspb.StringValue{
							Value: style,
						},
					},
				},
			},
		},
	}
	c := pb.NewCLIClientProxy(conn)

	stream, err := c.FDBCLIOneMany(ctx, req)

	if err != nil {
		return nil, fmt.Errorf("error while running %s: %v", style, err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("error while receiving stream: %v", err)
	}

	return resp, nil
}
