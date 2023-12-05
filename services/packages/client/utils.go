package client

import (
	"context"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/packages"
)

// InstallRemoteServiceMany is a helper function for installing binaries on multiple remote targets.
func InstallRemoteServiceMany(ctx context.Context, conn *proxy.Conn, system pb.PackageSystem, PackageName string, PackageVersion string, RepoEnabled string, RepoDisabled string) error {
	c := pb.NewPackagesClientProxy(conn)
	resp, err := c.InstallOneMany(ctx, &pb.InstallRequest{
		PackageSystem: system,
		Name:          PackageName,
		Version:       PackageVersion,
		Repo:          RepoEnabled,
		DisableRepo:   RepoDisabled,
	})

	if err != nil {
		return fmt.Errorf("can't install package %s - %v", PackageName, err)
	}

	errMsg := ""
	for r := range resp {
		if r.Error != nil {
			errMsg += fmt.Sprintf("can't install packages %s on target %s (%d): %v", PackageName, r.Target, r.Index, r.Error)
		}
	}
	if errMsg != "" {
		return fmt.Errorf("InstallRemoteServiceMany failed: %s", errMsg)
	}

	return nil
}
