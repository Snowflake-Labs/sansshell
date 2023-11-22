package client

import (
	"context"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/packages"
)

// InstallManyRemoteServices is a helper function for installing binaries on multiple remote targets.
func InstallManyRemoteServices(ctx context.Context, conn *proxy.Conn, system pb.PackageSystem, PackageName string, PackageVersion string, RepoEnabled string, RepoDisabled string) error {
	c := pb.NewPackagesClientProxy(conn)
	if _, err := c.InstallOneMany(ctx, &pb.InstallRequest{
		PackageSystem: system,
		Name:          PackageName,
		Version:       PackageVersion,
		Repo:          RepoEnabled,
		DisableRepo:   RepoDisabled,
	}); err != nil {
		return fmt.Errorf("can't install package %s - %v", PackageName, err)
	}
	return nil
}
