package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/service"
)

// RestartService is a helper function for restarting a service on a remote target
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func RestartService(ctx context.Context, conn *proxy.Conn, system pb.SystemType, service string) error {
	if len(conn.Targets) != 1 {
		return errors.New("RestartService only supports single targets")
	}

	c := pb.NewServiceClient(conn)
	if _, err := c.Action(ctx, &pb.ActionRequest{
		ServiceName: service,
		SystemType:  system,
		Action:      pb.Action_ACTION_RESTART,
	}); err != nil {
		return fmt.Errorf("can't restart service %s - %v", service, err)
	}
	return nil
}
