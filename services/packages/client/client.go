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

// Package client provides the client interface for 'packages'
package client

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/packages"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "packages"

func init() {
	subcommands.Register(&packagesCmd{}, subPackage)
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&installCmd{}, "")
	c.Register(&listCmd{}, "")
	c.Register(&repoListCmd{}, "")
	c.Register(&updateCmd{}, "")
	return c
}

type packagesCmd struct{}

func (*packagesCmd) Name() string { return subPackage }
func (p *packagesCmd) Synopsis() string {
	return client.GenerateSynopsis(setup(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *packagesCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*packagesCmd) SetFlags(f *flag.FlagSet) {}

func (p *packagesCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setup(f)
	return c.Execute(ctx, args...)
}

func flagToType(val string) (pb.PackageSystem, error) {
	v := fmt.Sprintf("PACKAGE_SYSTEM_%s", strings.ToUpper(val))
	i, ok := pb.PackageSystem_value[v]
	if !ok {
		return pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN, fmt.Errorf("no such sumtype value: %s", v)
	}
	return pb.PackageSystem(i), nil
}

func shortPackageSystemNames() []string {
	var shortNames []string
	for k := range pb.PackageSystem_value {
		shortNames = append(shortNames, strings.TrimPrefix(k, "PACKAGE_SYSTEM_"))
	}
	sort.Strings(shortNames)
	return shortNames
}

type installCmd struct {
	packageSystem string
	name          string
	version       string
	repo          string
	disable       string
}

func (*installCmd) Name() string     { return "install" }
func (*installCmd) Synopsis() string { return "Install a new package" }
func (*installCmd) Usage() string {
	return `install [--package_system=P] --name=X --version=Y [--disablerepo=A] [--repo|enablerepo=Z]:
  Install a new package on the remote machine.
`
}

func (i *installCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&i.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.StringVar(&i.name, "name", "", "Name of package to install")
	f.StringVar(&i.version, "version", "", "Version of package to install. For YUM this must be a full nevra version")
	f.StringVar(&i.repo, "repo", "", "If set also enables this repo when resolving packages.")
	f.StringVar(&i.repo, "enablerepo", "", "If set also enables this repo when resolving packages.")
	f.StringVar(&i.disable, "disablerepo", "", "If set also disables this repo when resolving packages.")
}

func (i *installCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	ps, err := flagToType(i.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", i.packageSystem)
		return subcommands.ExitFailure
	}

	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "All options are set via flags")
		return subcommands.ExitFailure
	}
	if i.name == "" || i.version == "" {
		fmt.Fprintln(os.Stderr, "Both --name and --version must be filled in")
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClientProxy(state.Conn)

	req := &pb.InstallRequest{
		PackageSystem: ps,
		Name:          i.name,
		Version:       i.version,
		Repo:          i.repo,
		DisableRepo:   i.disable,
	}

	resp, err := c.InstallOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - Install returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Install for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprintf(state.Out[r.Index], "Success!\n\nOutput from installation:\n%s\n", r.Resp.DebugOutput)
	}
	return retCode
}

type updateCmd struct {
	packageSystem string
	name          string
	oldVersion    string
	newVersion    string
	repo          string
	disable       string
}

func (*updateCmd) Name() string     { return "update" }
func (*updateCmd) Synopsis() string { return "Update an existing package" }
func (*updateCmd) Usage() string {
	return `update [--package_system=P] --name=X --old_version=Y --new_version=Z [--disablerepo=B] [--repo|enablerepo=A]:
  Update a package on the remote machine. The package must already be installed at a known version.
`
}

func (u *updateCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&u.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.StringVar(&u.name, "name", "", "Name of package to install")
	f.StringVar(&u.oldVersion, "old_version", "", "Old version of package which must be on the system. For YUM this must be a full nevra version")
	f.StringVar(&u.newVersion, "new_version", "", "New version of package to update. For YUM this must be a full nevra version")
	f.StringVar(&u.repo, "repo", "", "If set also enables this repo when resolving packages.")
	f.StringVar(&u.repo, "enablerepo", "", "If set also enables this repo when resolving packages.")
	f.StringVar(&u.disable, "disablerepo", "", "If set also disables this repo when resolving packages.")
}

func (u *updateCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "All options are set via flags")
		return subcommands.ExitFailure
	}
	if u.name == "" || u.oldVersion == "" || u.newVersion == "" {
		fmt.Fprintln(os.Stderr, "--name, --old_version and --new_version must be supplied")
		return subcommands.ExitFailure
	}

	ps, err := flagToType(u.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", u.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClientProxy(state.Conn)

	req := &pb.UpdateRequest{
		PackageSystem: ps,
		Name:          u.name,
		OldVersion:    u.oldVersion,
		NewVersion:    u.newVersion,
		Repo:          u.repo,
		DisableRepo:   u.disable,
	}

	resp, err := c.UpdateOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - Update returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Update for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprintf(state.Out[r.Index], "Success!\n\nOutput from update:\n%s\n", r.Resp.DebugOutput)
	}
	return retCode
}

type listCmd struct {
	packageSystem string
}

func (*listCmd) Name() string     { return "list" }
func (*listCmd) Synopsis() string { return "List installed packages" }
func (*listCmd) Usage() string {
	return `list [--package_system=P]:
  List the installed packages on the remote machine.
`
}

func (l *listCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&l.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
}

func (l *listCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "All options are set via flags")
		return subcommands.ExitFailure
	}
	ps, err := flagToType(l.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", l.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClientProxy(state.Conn)

	resp, err := c.ListInstalledOneMany(ctx, &pb.ListInstalledRequest{
		PackageSystem: ps,
	})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - List returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Update for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprint(state.Out[r.Index], "Installed Packages\n")
		for _, pkg := range r.Resp.Packages {
			// Print the package name, version and repo with some reasonable spacing.
			fmt.Fprintf(state.Out[r.Index], "%40s %16s %32s\n", pkg.Name, pkg.Version, pkg.Repo)
		}
	}
	return retCode
}

type repoListCmd struct {
	packageSystem string
	verbose       bool
}

func (*repoListCmd) Name() string     { return "repolist" }
func (*repoListCmd) Synopsis() string { return "List repos defined on machine" }
func (*repoListCmd) Usage() string {
	return `repolist [--package_system=P]:
  List the repos defined on the remote machine.
`
}

func (r *repoListCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&r.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.BoolVar(&r.verbose, "verbose", false, "If true print out fully verbose outage")
}

func (r *repoListCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "All options are set via flags")
		return subcommands.ExitFailure
	}
	ps, err := flagToType(r.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", r.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClientProxy(state.Conn)

	resp, err := c.RepoListOneMany(ctx, &pb.RepoListRequest{
		PackageSystem: ps,
	})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - Repo list returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for s := range resp {
		if s.Error != nil {
			fmt.Fprintf(state.Err[s.Index], "Repo list for target %s (%d) returned error: %v\n", s.Target, s.Index, s.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		// Print the repo id, name and status with some reasonable spacing.
		if r.verbose {
			// Print like "yum repolist all -v would.
			for _, repo := range s.Resp.Repos {
				fmt.Fprintf(state.Out[s.Index], "Repo-id      : %s\n", repo.Id)
				fmt.Fprintf(state.Out[s.Index], "Repo-name    : %s\n", repo.Name)
				fmt.Fprintf(state.Out[s.Index], "Repo-status  : %s\n", getStatus(repo.Status))
				fmt.Fprintf(state.Out[s.Index], "Repo-baseurl : %s\n", repo.Url)
				fmt.Fprintf(state.Out[s.Index], "Repo-filename: %s\n", repo.Filename)
				fmt.Fprintln(state.Out[s.Index])
			}
		} else {
			format := "%35s %65s %10s\n"
			fmt.Fprintf(state.Out[s.Index], format, "repo id", "repo name", "status")
			for _, repo := range s.Resp.Repos {
				fmt.Fprintf(state.Out[s.Index], format, repo.Id, repo.Name, getStatus(repo.Status))
			}
		}
	}
	return retCode
}

func getStatus(s pb.RepoStatus) string {
	status := "unknown"
	switch s {
	case pb.RepoStatus_REPO_STATUS_DISABLED:
		status = "disabled"
	case pb.RepoStatus_REPO_STATUS_ENABLED:
		status = "enabled"
	}
	return status
}
