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

func (*packagesCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&installCmd{}, "")
	c.Register(&removeCmd{}, "")
	c.Register(&listCmd{}, "")
	c.Register(&repoListCmd{}, "")
	c.Register(&updateCmd{}, "")
	c.Register(&cleanupCmd{}, "")
	c.Register(&searchCmd{}, "")
	return c
}

type packagesCmd struct{}

func (*packagesCmd) Name() string { return subPackage }
func (p *packagesCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *packagesCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*packagesCmd) SetFlags(f *flag.FlagSet) {}

func (p *packagesCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
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

  Version format is NEVRA without the name, like --name bash --new_version 0:4.2.46-34.el7.x86_64
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
		subcommands.DefaultCommander.ExplainCommand(os.Stderr, i)
		return subcommands.ExitUsageError
	}
	if i.name == "" || i.version == "" {
		fmt.Fprintln(os.Stderr, "Both --name and --version must be filled in")
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	//TODO: Unresolved reference, missing import util?
	err = InstallManyRemoteServices(ctx, state.Conn, ps, i.name, i.version, i.repo, i.disable)

	// Comment out the following code temporarily for testing utils
	//c := pb.NewPackagesClientProxy(state.Conn)
	//
	//req := &pb.InstallRequest{
	//	PackageSystem: ps,
	//	Name:          i.name,
	//	Version:       i.version,
	//	Repo:          i.repo,
	//	DisableRepo:   i.disable,
	//}

	//resp, err := c.InstallOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		//for _, e := range state.Err {
		//	fmt.Fprintf(e, "All targets - Install returned error: %v\n", err)
		//}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	//for r := range resp {
	//	if r.Error != nil {
	//		fmt.Fprintf(state.Err[r.Index], "Install for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
	//		retCode = subcommands.ExitFailure
	//		continue
	//	}
	//	fmt.Fprintf(state.Out[r.Index], "Success!\n\nOutput from installation:\n%s\n", r.Resp.DebugOutput)
	//}
	return retCode
}

type removeCmd struct {
	packageSystem string
	name          string
	version       string
}

func (*removeCmd) Name() string     { return "remove" }
func (*removeCmd) Synopsis() string { return "Remove installed package" }
func (*removeCmd) Usage() string {
	return `remove [--package_system=P] --name=X --version=Y
  Remove installed package on the remote machine.
 `
}

func (r *removeCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&r.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.StringVar(&r.name, "name", "", "Name of package to remove")
	f.StringVar(&r.version, "version", "", "Version of package to remove. For YUM this must be a full nevra version")
}

func (r *removeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	ps, err := flagToType(r.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", r.packageSystem)
		return subcommands.ExitFailure
	}

	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "All options are set via flags")
		subcommands.DefaultCommander.ExplainCommand(os.Stderr, r)
		return subcommands.ExitUsageError
	}
	if r.name == "" || r.version == "" {
		fmt.Fprintln(os.Stderr, "Both --name and --version must be filled in")
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClientProxy(state.Conn)

	req := &pb.RemoveRequest{
		PackageSystem: ps,
		Name:          r.name,
		Version:       r.version,
	}

	resp, err := c.RemoveOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - Remove returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Remove for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
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

  Version format is NEVRA without the name, like -name bash -old_version 0:4.2.46-34.el7.x86_64 -new_version 0:4.2.46-35.el7_9.x86_64
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
		subcommands.DefaultCommander.ExplainCommand(os.Stderr, u)
		return subcommands.ExitUsageError
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
	nevra         bool
}

func (*listCmd) Name() string     { return "list" }
func (*listCmd) Synopsis() string { return "List installed packages" }
func (*listCmd) Usage() string {
	return `list [--package_system=P] [--nevra]:
  List the installed packages on the remote machine.
`
}

func (l *listCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&l.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.BoolVar(&l.nevra, "nevra", false, "For YUM, print output in NEVRA format instead of trying to imitate `yum list-installed`")
}

func (l *listCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "All options are set via flags")
		subcommands.DefaultCommander.ExplainCommand(os.Stderr, l)
		return subcommands.ExitUsageError
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
		if l.nevra {
			for _, pkg := range r.Resp.Packages {
				ne := pkg.Name
				if pkg.Epoch != 0 {
					ne = fmt.Sprintf("%s-%d", pkg.Name, pkg.Epoch)
				}
				nevra := fmt.Sprintf("%v:%v-%v.%v", ne, pkg.Version, pkg.Release, pkg.Architecture)
				fmt.Fprintln(state.Out[r.Index], nevra)
			}
		} else {
			fmt.Fprint(state.Out[r.Index], "Installed Packages\n")
			for _, pkg := range r.Resp.Packages {
				// Print the package name.arch, [epoch]:version-release and repo with some reasonable spacing.
				na := pkg.Name
				if pkg.Architecture != "" {
					na = na + "." + pkg.Architecture
				}
				evr := pkg.Version
				if pkg.Release != "" {
					evr = evr + "-" + pkg.Release
				}
				if pkg.Epoch != 0 {
					evr = fmt.Sprintf("%d:%s", pkg.Epoch, evr)
				}
				fmt.Fprintf(state.Out[r.Index], "%40s %16s %32s\n", na, evr, pkg.Repo)
			}
		}
	}
	return retCode
}

type searchCmd struct {
	packageSystem string
	name          string
	installed     bool
	available     bool
}

func (*searchCmd) Name() string     { return "search" }
func (*searchCmd) Synopsis() string { return "Search NEVRA of a package" }
func (*searchCmd) Usage() string {
	return `search --name=X [--installed] [--available] [--package_system=P]:
  Search the current installed packages or all available packages in package repo. The packages will be displayed in NEVRA version.
  By default, --installed and --available will be enabled.
`
}

func (l *searchCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&l.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.StringVar(&l.name, "name", "", "Name of package to search")
	f.BoolVar(&l.installed, "installed", false, "If true print out installed NEVRA of the package")
	f.BoolVar(&l.available, "available", false, "If true print out all available NEVRA of the package")
}

func (l *searchCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "All options are set via flags")
		subcommands.DefaultCommander.ExplainCommand(os.Stderr, l)
		return subcommands.ExitUsageError
	}
	if l.name == "" {
		fmt.Fprintln(os.Stderr, "--name must be supplied")
		return subcommands.ExitFailure
	}

	// if we don't set flags for current and latest, both should be set by default
	if !l.installed && !l.available {
		l.installed, l.available = true, true
	}

	ps, err := flagToType(l.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", l.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClientProxy(state.Conn)

	resp, err := c.SearchOneMany(ctx, &pb.SearchRequest{
		PackageSystem: ps,
		Name:          l.name,
		Installed:     l.installed,
		Available:     l.available,
	})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - Search returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "Search for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		if l.installed {
			fmt.Fprintf(state.Out[r.Index], "%s:\n", "Installed Packages")
			if packages := r.Resp.InstalledPackages.Packages; len(packages) == 0 {
				fmt.Fprintf(state.Out[r.Index], "%-10s%s\n", " ", "No package found!")
			} else {
				for _, packageInfo := range packages {
					fmt.Fprintf(state.Out[r.Index], "%-10s%s-%d:%s-%s.%s\n", " ", packageInfo.Name, packageInfo.Epoch, packageInfo.Version, packageInfo.Release, packageInfo.Architecture)
				}
			}

		}
		if l.available {
			fmt.Fprintf(state.Out[r.Index], "%s:\n", "Available Packages")
			if packages := r.Resp.AvailablePackages.Packages; len(packages) == 0 {
				fmt.Fprintf(state.Out[r.Index], "%-10s%s\n", " ", "No package found!")
			} else {
				for _, packageInfo := range packages {
					fmt.Fprintf(state.Out[r.Index], "%-10s%s-%d:%s-%s.%s\n", " ", packageInfo.Name, packageInfo.Epoch, packageInfo.Version, packageInfo.Release, packageInfo.Architecture)
				}
			}
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
		subcommands.DefaultCommander.ExplainCommand(os.Stderr, r)
		return subcommands.ExitUsageError
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

type cleanupCmd struct {
	packageSystem string
}

func (*cleanupCmd) Name() string     { return "cleanup" }
func (*cleanupCmd) Synopsis() string { return "Run cleanup on machine" }
func (*cleanupCmd) Usage() string {
	return `cleanup [--package_system=P]:
  Run cleanup on remote machine
`
}

func (r *cleanupCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&r.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
}

func (r *cleanupCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "All options are set via flags")
		subcommands.DefaultCommander.ExplainCommand(os.Stderr, r)
		return subcommands.ExitUsageError
	}
	ps, err := flagToType(r.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", r.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClientProxy(state.Conn)

	resp, err := c.CleanupOneMany(ctx, &pb.CleanupRequest{
		PackageSystem: ps,
	})
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - Cleanup returned error: %v\n", err)
		}
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for s := range resp {
		if s.Error != nil {
			fmt.Fprintf(state.Err[s.Index], "Cleanup for target %s (%d) returned error: %v\n", s.Target, s.Index, s.Error)
			retCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprintf(state.Out[s.Index], "%s", s.Resp.DebugOutput)
	}
	return retCode
}
