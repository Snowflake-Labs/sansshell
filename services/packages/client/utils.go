package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/packages"
)

// InstallManyRemoteServices is a helper function for installing binaries on multiple remote targets.
func InstallManyRemoteServices(ctx context.Context, conn *proxy.Conn, system pb.PackageSystem, PackageName string, PackageVersion string, RepoEnabled string, RepoDisabled string) error {
	c := pb.NewPackagesClientProxy(conn)
	respChan, err := c.InstallOneMany(ctx, &pb.InstallRequest{
		PackageSystem: system,
		Name:          PackageName,
		Version:       PackageVersion,
		Repo:          RepoEnabled,
		DisableRepo:   RepoDisabled,
	})

	if err != nil {
		return fmt.Errorf("can't install package %s - %v", PackageName, err)
	}

	var errorList []error
	for resp := range respChan {
		if resp.Error != nil {
			err := fmt.Errorf("can't install package %s on target %s: %w", PackageName, resp.Target, resp.Error)
			errorList = append(errorList, err)
		}
	}

	if len(errorList) == 0 {
		return nil
	}

	var errorMessages []string
	for _, err := range errorList {
		errorMessages = append(errorMessages, err.Error())
	}

	return fmt.Errorf("%s", strings.Join(errorMessages, "\n"))
}
