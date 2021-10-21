package client

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	pb "github.com/Snowflake-Labs/sansshell/services/packages"
)

func init() {
	subcommands.Register(&installCmd{}, "raw")
	subcommands.Register(&updateCmd{}, "raw")
	subcommands.Register(&listCmd{}, "raw")
	subcommands.Register(&repoListCmd{}, "raw")
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
	f.StringVar(&i.packageSystem, "package_system", "YUM", "Either blank or YUM (which means the same thing currently)")
	f.StringVar(&i.name, "name", "", "Name of package to install")
	f.StringVar(&i.version, "version", "", "Version of package to install. For YUM this must be a full nevra version")
	f.StringVar(&i.repo, "repo", "", "If set also enable this repo when resolving packages.")
}

func (i *installCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if i.packageSystem != "YUM" && i.packageSystem != "" {
		fmt.Fprint(os.Stderr, "--package_system must be blank or YUM")
		return subcommands.ExitFailure
	}

	conn := args[0].(*grpc.ClientConn)

	c := pb.NewPackagesClient(conn)

	req := &pb.InstallRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
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
	f.StringVar(&u.packageSystem, "package_system", "YUM", "Either blank or YUM (which means the same thing currently)")
	f.StringVar(&u.name, "name", "", "Name of package to install")
	f.StringVar(&u.old_version, "old_version", "", "Old version of package which must be on the system. For YUM this must be a full nevra version")
	f.StringVar(&u.new_version, "new_version", "", "New version of package to update. For YUM this must be a full nevra version")
	f.StringVar(&u.repo, "repo", "", "If set also enable this repo when resolving packages.")
}

func (u *updateCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if u.packageSystem != "YUM" && u.packageSystem != "" {
		fmt.Fprint(os.Stderr, "--package_system must be blank or YUM")
		return subcommands.ExitFailure
	}

	conn := args[0].(*grpc.ClientConn)

	c := pb.NewPackagesClient(conn)

	req := &pb.UpdateRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
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
	f.StringVar(&l.packageSystem, "package_system", "YUM", "Either blank or YUM (which means the same thing currently)")
}

func (l *listCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if l.packageSystem != "YUM" && l.packageSystem != "" {
		fmt.Fprint(os.Stderr, "--package_system must be blank or YUM")
		return subcommands.ExitFailure
	}

	conn := args[0].(*grpc.ClientConn)

	c := pb.NewPackagesClient(conn)

	resp, err := c.ListInstalled(ctx, &pb.ListInstalledRequest{})
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
}

func (*repoListCmd) Name() string     { return "repolist" }
func (*repoListCmd) Synopsis() string { return "List repos defined on machine" }
func (*repoListCmd) Usage() string {
	return `repolist:
  List the repos defined on the remote machine.
`
}

func (r *repoListCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&r.packageSystem, "package_system", "YUM", "Either blank or YUM (which means the same thing currently)")
}

func (r *repoListCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if r.packageSystem != "YUM" && r.packageSystem != "" {
		fmt.Fprint(os.Stderr, "--package_system must be blank or YUM")
		return subcommands.ExitFailure
	}

	conn := args[0].(*grpc.ClientConn)

	c := pb.NewPackagesClient(conn)

	resp, err := c.RepoList(ctx, &pb.RepoListRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Repo list returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	// Print the repo id, name and status with some reasonable spacing.
	format := "%40s %40s %16s\n"
	fmt.Fprint(os.Stdout, format, "repo id", "repo name", "status")
	for _, repo := range resp.Repos {
		status := "unknown"
		switch repo.Status {
		case pb.RepoStatus_REPO_STATUS_DISABLED:
			status = "disabled"
		case pb.RepoStatus_REPO_STATUS_ENABLED:
			status = "enabled"
		}
		fmt.Fprintf(os.Stdout, format, repo.Id, repo.Name, status)
	}
	return subcommands.ExitSuccess
}
