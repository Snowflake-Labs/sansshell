package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/service"
)

// ListRemoteServices is a helper function for listing all services on a remote target
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func ListRemoteServices(ctx context.Context, conn *proxy.Conn, system pb.SystemType) (*pb.ListReply, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("ListRemoteServices only supports single targets")
	}

	c := pb.NewServiceClient(conn)
	ret, err := c.List(ctx, &pb.ListRequest{
		SystemType: system,
	})
	if err != nil {
		return nil, fmt.Errorf("can't list services %v", err)
	}
	return ret, nil
}

// StatusRemoteService is a helper function for getting the status of a service on a remote target
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func StatusRemoteService(ctx context.Context, conn *proxy.Conn, system pb.SystemType, service string) (*pb.StatusReply, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("StatusRemoteService only supports single targets")
	}

	c := pb.NewServiceClient(conn)
	ret, err := c.Status(ctx, &pb.StatusRequest{
		SystemType:  system,
		ServiceName: service,
	})
	if err != nil {
		return nil, fmt.Errorf("can't get status for service %s - %v", service, err)
	}
	return ret, nil
}

// StartRemoteService is a helper function for starting a service on a remote target
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func StartRemoteService(ctx context.Context, conn *proxy.Conn, system pb.SystemType, service string) error {
	if len(conn.Targets) != 1 {
		return errors.New("StartRemoteService only supports single targets")
	}

	c := pb.NewServiceClient(conn)
	if _, err := c.Action(ctx, &pb.ActionRequest{
		ServiceName: service,
		SystemType:  system,
		Action:      pb.Action_ACTION_START,
	}); err != nil {
		return fmt.Errorf("can't start service %s - %v", service, err)
	}
	return nil
}

// StopRemoteService is a helper function for stopping a service on a remote target
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func StopRemoteService(ctx context.Context, conn *proxy.Conn, system pb.SystemType, service string) error {
	if len(conn.Targets) != 1 {
		return errors.New("StopRemoteService only supports single targets")
	}

	c := pb.NewServiceClient(conn)
	if _, err := c.Action(ctx, &pb.ActionRequest{
		ServiceName: service,
		SystemType:  system,
		Action:      pb.Action_ACTION_STOP,
	}); err != nil {
		return fmt.Errorf("can't stop service %s - %v", service, err)
	}
	return nil
}

// RestartService was the original exported name for RestartRemoteService and now
// exists for backwards compatibility.
//
// Deprecated: Use RestartRemoteService instead.
var RestartService = RestartRemoteService

// RestartRemoteService is a helper function for restarting a service on a remote target
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func RestartRemoteService(ctx context.Context, conn *proxy.Conn, system pb.SystemType, service string) error {
	if len(conn.Targets) != 1 {
		return errors.New("RestartRemoteService only supports single targets")
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

// EnableRemoteService is a helper function for enabling a service on a remote target
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func EnableRemoteService(ctx context.Context, conn *proxy.Conn, system pb.SystemType, service string) error {
	if len(conn.Targets) != 1 {
		return errors.New("EnableRemoteService only supports single targets")
	}

	c := pb.NewServiceClient(conn)
	if _, err := c.Action(ctx, &pb.ActionRequest{
		ServiceName: service,
		SystemType:  system,
		Action:      pb.Action_ACTION_ENABLE,
	}); err != nil {
		return fmt.Errorf("can't enable service %s - %v", service, err)
	}
	return nil
}

// DisableRemoteService is a helper function for disabling a service on a remote target
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func DisableRemoteService(ctx context.Context, conn *proxy.Conn, system pb.SystemType, service string) error {
	if len(conn.Targets) != 1 {
		return errors.New("DisableRemoteService only supports single targets")
	}

	c := pb.NewServiceClient(conn)
	if _, err := c.Action(ctx, &pb.ActionRequest{
		ServiceName: service,
		SystemType:  system,
		Action:      pb.Action_ACTION_DISABLE,
	}); err != nil {
		return fmt.Errorf("can't disable service %s - %v", service, err)
	}
	return nil
}
