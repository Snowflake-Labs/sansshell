package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/exec"
)

// ExecRemoteCommand is a helper function for execing a command on a remote host
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func ExecRemoteCommand(ctx context.Context, conn *proxy.Conn, binary string, args ...string) (*pb.ExecResponse, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("ExecRemoteCommand only supports single targets")
	}

	result, err := ExecRemoteCommandMany(ctx, conn, binary, args...)
	if err != nil {
		return nil, err
	}
	if len(result) < 1 {
		return nil, fmt.Errorf("ExecRemoteCommand error: received empty response")
	}
	return result[0].ExecResponse, nil
}

type ExecResponse struct {
	*pb.ExecResponse
	Index  int
	Target string
}

func ExecRemoteCommandMany(ctx context.Context, conn *proxy.Conn, binary string, args ...string) ([]ExecResponse, error) {
	c := pb.NewExecClientProxy(conn)
	req := &pb.ExecRequest{
		Command: binary,
		Args:    args,
	}
	respChan, err := c.RunOneMany(ctx, req)
	if err != nil {
		return nil, err
	}
	result := make([]ExecResponse, len(conn.Targets))
	for r := range respChan {
		result[r.Index] = ExecResponse{
			r.Resp,
			r.Index,
			r.Target,
		}
	}

	return result, nil
}
