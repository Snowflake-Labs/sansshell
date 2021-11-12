package client

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/subcommands"

	pb "github.com/Snowflake-Labs/sansshell/services/packages"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

func init() {
	subcommands.Register(&installCmd{}, "packages")
	subcommands.Register(&updateCmd{}, "packages")
	subcommands.Register(&listCmd{}, "packages")
	subcommands.Register(&repoListCmd{}, "packages")
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
}

func (*installCmd) Name() string     { return "install" }
func (*installCmd) Synopsis() string { return "Install a new package" }
func (*installCmd) Usage() string {
	return `install:
  Install a new package on the remote machine.
`
}

func (i *installCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&i.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.StringVar(&i.name, "name", "", "Name of package to install")
	f.StringVar(&i.version, "version", "", "Version of package to install. For YUM this must be a full nevra version")
	f.StringVar(&i.repo, "repo", "", "If set also enable this repo when resolving packages.")
}

func (i *installCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	ps, err := flagToType(i.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", i.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClient(state.Conn)

	req := &pb.InstallRequest{
		PackageSystem: ps,
		Name:          i.name,
		Version:       i.version,
		Repo:          i.repo,
	}

	resp, err := c.Install(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Install returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	fmt.Fprintf(os.Stdout, "Success!\n\nOutput from installation:\n%s\n", resp.DebugOutput)
	return subcommands.ExitSuccess
}

type updateCmd struct {
	packageSystem string
	name          string
	old_version   string
	new_version   string
	repo          string
}

func (*updateCmd) Name() string     { return "update" }
func (*updateCmd) Synopsis() string { return "Update an existing package" }
func (*updateCmd) Usage() string {
	return `update:
  Update a package on the remote machine. The package must already be installed at a known version.
`
}

func (u *updateCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&u.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.StringVar(&u.name, "name", "", "Name of package to install")
	f.StringVar(&u.old_version, "old_version", "", "Old version of package which must be on the system. For YUM this must be a full nevra version")
	f.StringVar(&u.new_version, "new_version", "", "New version of package to update. For YUM this must be a full nevra version")
	f.StringVar(&u.repo, "repo", "", "If set also enable this repo when resolving packages.")
}

func (u *updateCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if u.name == "" || u.old_version == "" || u.new_version == "" {
		fmt.Fprintln(os.Stderr, "--name, --old_version and --new_version must be supplied")
		return subcommands.ExitFailure
	}

	ps, err := flagToType(u.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", u.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClient(state.Conn)

	req := &pb.UpdateRequest{
		PackageSystem: ps,
		Name:          u.name,
		OldVersion:    u.old_version,
		NewVersion:    u.new_version,
		Repo:          u.repo,
	}

	resp, err := c.Update(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	fmt.Fprintf(os.Stdout, "Success!\n\nOutput from update:\n%s\n", resp.DebugOutput)
	return subcommands.ExitSuccess
}

type listCmd struct {
	packageSystem string
}

func (*listCmd) Name() string     { return "list" }
func (*listCmd) Synopsis() string { return "List installed packages" }
func (*listCmd) Usage() string {
	return `list:
  List the installed packages on the remote machine.
`
}

func (l *listCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&l.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
}

func (l *listCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	ps, err := flagToType(l.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", l.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClient(state.Conn)

	resp, err := c.ListInstalled(ctx, &pb.ListInstalledRequest{
		PackageSystem: ps,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "List returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	fmt.Fprint(os.Stdout, "Installed Packages\n")
	for _, pkg := range resp.Packages {
		// Print the package name, version and repo with some reasonable spacing.
		fmt.Fprintf(os.Stdout, "%40s %16s %32s\n", pkg.Name, pkg.Version, pkg.Repo)
	}
	return subcommands.ExitSuccess
}

type repoListCmd struct {
	packageSystem string
	verbose       bool
}

func (*repoListCmd) Name() string     { return "repolist" }
func (*repoListCmd) Synopsis() string { return "List repos defined on machine" }
func (*repoListCmd) Usage() string {
	return `repolist:
  List the repos defined on the remote machine.
`
}

func (r *repoListCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&r.packageSystem, "package-system", "YUM", fmt.Sprintf("Package system to use(one of: [%s])", strings.Join(shortPackageSystemNames(), ",")))
	f.BoolVar(&r.verbose, "verbose", false, "If true print out fully verbose outage")
}

func (r *repoListCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	ps, err := flagToType(r.packageSystem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse package system for --package-system: %s invalid\n", r.packageSystem)
		return subcommands.ExitFailure
	}

	state := args[0].(*util.ExecuteState)
	c := pb.NewPackagesClient(state.Conn)

	resp, err := c.RepoList(ctx, &pb.RepoListRequest{
		PackageSystem: ps,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Repo list returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	// Print the repo id, name and status with some reasonable spacing.
	if r.verbose {
		// Print like "yum repolist all -v would.
		for _, repo := range resp.Repos {
			fmt.Fprintf(os.Stdout, "Repo-id      : %s\n", repo.Id)
			fmt.Fprintf(os.Stdout, "Repo-name    : %s\n", repo.Name)
			fmt.Fprintf(os.Stdout, "Repo-status  : %s\n", getStatus(repo.Status))
			fmt.Fprintf(os.Stdout, "Repo-baseurl : %s\n", repo.Url)
			fmt.Fprintf(os.Stdout, "Repo-filename: %s\n", repo.Filename)
			fmt.Fprintln(os.Stdout)
		}
	} else {
		format := "%35s %65s %10s\n"
		fmt.Fprintf(os.Stdout, format, "repo id", "repo name", "status")
		for _, repo := range resp.Repos {
			fmt.Fprintf(os.Stdout, format, repo.Id, repo.Name, getStatus(repo.Status))
		}
	}
	return subcommands.ExitSuccess
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
