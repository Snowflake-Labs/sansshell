package client

import (
	"context"
	"errors"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/exec"
)

// ExecRemoteCommand is a helper function for execing a command on a remote host
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func ExecRemoteCommand(ctx context.Context, conn *proxy.Conn, binary string, args ...string) (*pb.ExecResponse, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("ExecRemoteCommand only supports single targets")
	}

	c := pb.NewExecClient(conn)
	return c.Run(ctx, &pb.ExecRequest{
		Command: binary,
		Args:    args,
	})
}
