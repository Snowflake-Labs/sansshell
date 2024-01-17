/* Copyright (c) 2019 Snowflake Inc. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

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
