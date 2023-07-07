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

// Package server implements the sansshell 'Packages' service.
package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/packages"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
)

// Metrics
var (
	packagesSearchFailureCounter = metrics.MetricDefinition{Name: "actions_packages_search_failure",
		Description: "number of failures when performing packages.Search"}
	packagesListInstalledFailureCounter = metrics.MetricDefinition{Name: "actions_packages_listinstalled_failure",
		Description: "number of failures when performing packages.ListInstalled"}
	packagesRepoListFailureCounter = metrics.MetricDefinition{Name: "actions_packages_repolist_failure",
		Description: "number of failures when performing packages.RepoList"}
	packagesCleanupFailureCounter = metrics.MetricDefinition{Name: "actions_packages_cleanup_failure",
		Description: "number of failures when performing packages.Cleanup"}
	packagesInstallFailureCounter = metrics.MetricDefinition{Name: "actions_packages_install_failure",
		Description: "number of failures when performing packages.Install"}
	packagesUpdateFailureCounter = metrics.MetricDefinition{Name: "actions_packages_update_failure",
		Description: "number of failures when performing packages.Update"}
	packagesRemoveFailureCounter = metrics.MetricDefinition{Name: "actions_packages_remove_failure",
		Description: "number of failures when performing packages.Remove"}
)

// Internal helper to generate the command list. The map must contain the enum.
func genCmd(p pb.PackageSystem, m map[pb.PackageSystem][]string) ([]string, error) {
	var out []string
	switch p {
	case pb.PackageSystem_PACKAGE_SYSTEM_YUM:
		if YumBin == "" {
			return nil, status.Errorf(codes.Unimplemented, "no support for yum")
		}
		out = append(out, YumBin)
		out = append(out, m[p]...)
	default:
		return nil, status.Errorf(codes.Unimplemented, "no support for package system enum %d", p)
	}
	return out, nil
}

type repoData struct {
	enable  string
	disable string
}

// Optionally add the repo arg and then append the full package name to the list.
func addRepoAndPackage(out []string, p pb.PackageSystem, name string, version string, repos *repoData) []string {
	// Disable must go first since it'll over enable otherwise and leave no repos potentially
	if repos.disable != "" && p == pb.PackageSystem_PACKAGE_SYSTEM_YUM {
		out = append(out, fmt.Sprintf("--disablerepo=%s", repos.disable))
	}
	if repos.enable != "" && p == pb.PackageSystem_PACKAGE_SYSTEM_YUM {
		out = append(out, fmt.Sprintf("--enablerepo=%s", repos.enable))
	}
	// Tack the fully qualfied package name on. This assumes any vetting of args has already been done.
	out = append(out, fmt.Sprintf("%s-%s", name, version))
	return out
}

var (
	inputValidateRe = regexp.MustCompile("[^a-zA-Z0-9_.:-]+")

	// These are vars for testing to be able to replace them.
	generateInstall = func(p *pb.InstallRequest) ([]string, error) {
		installOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"install-nevra",
				"-y",
			},
		}
		out, err := genCmd(p.PackageSystem, installOpts)
		if err != nil {
			return nil, err
		}
		repos := &repoData{
			enable:  p.Repo,
			disable: p.DisableRepo,
		}
		return addRepoAndPackage(out, p.PackageSystem, p.Name, p.Version, repos), nil
	}

	generateRemove = func(p *pb.RemoveRequest) ([]string, error) {
		removeOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"remove-nevra",
				"-y",
			},
		}
		return genCmd(p.PackageSystem, removeOpts)
	}

	generateValidate = func(p *pb.UpdateRequest) ([]string, error) {
		validateOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"list",
				"installed",
			},
		}
		out, err := genCmd(p.PackageSystem, validateOpts)
		if err != nil {
			return nil, err
		}
		return addRepoAndPackage(out, p.PackageSystem, p.Name, p.OldVersion, &repoData{}), nil
	}

	generateUpdate = func(p *pb.UpdateRequest) ([]string, error) {
		updateOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"update-to",
				"-y",
			},
		}
		out, err := genCmd(p.PackageSystem, updateOpts)
		if err != nil {
			return nil, err
		}
		repos := &repoData{
			enable:  p.Repo,
			disable: p.DisableRepo,
		}
		return addRepoAndPackage(out, p.PackageSystem, p.Name, p.NewVersion, repos), nil
	}

	generateListInstalled = func(p pb.PackageSystem) ([]string, error) {
		listOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"list",
				"installed",
			},
		}
		return genCmd(p, listOpts)
	}

	generateSearch = func(p *pb.SearchRequest, searchInstalled bool) ([]string, error) {
		var out []string
		switch p.PackageSystem {
		case pb.PackageSystem_PACKAGE_SYSTEM_YUM:
			out = append(out, RepoqueryBin)
			out = append(out, "--nevra")
			out = append(out, p.Name)
			if searchInstalled {
				out = append(out, "--installed")
			}
		default:
			return nil, status.Errorf(codes.Unimplemented, "no support for package system enum %d", p.PackageSystem)
		}

		return out, nil
	}

	generateRepoList = func(p pb.PackageSystem) ([]string, error) {
		repoOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"repoinfo",
				"all",
			},
		}
		return genCmd(p, repoOpts)
	}

	generateCleanup = func(p pb.PackageSystem) ([]string, error) {
		var out []string
		switch p {
		case pb.PackageSystem_PACKAGE_SYSTEM_YUM:
			if YumCleanup == "" {
				return nil, status.Errorf(codes.Unimplemented, "no support for yum-complete-transaction")
			}
			out = append(out, YumCleanup)
			out = append(out, "--cleanup-only")
		default:
			return nil, status.Errorf(codes.Unimplemented, "no support for package system enum %d", p)
		}
		return out, nil
	}

	needRebootRunCmd = func(ctx context.Context) (bool, error) {
		run, err := util.RunCommand(ctx, NeedsRestartingBin, []string{"--reboothint"})
		if err != nil {
			return false, err
		}

		return run.ExitCode == 1, nil
	}
)

// server is used to implement the gRPC server
type server struct{}

func validateField(param string, name string) error {
	if len(name) == 0 {
		return status.Errorf(codes.InvalidArgument, "%s must be filled in", param)
	}
	if strings.HasPrefix(name, "-") {
		return status.Errorf(codes.InvalidArgument, "package %s %q invalid. Cannot start with a dash", param, name)
	}
	if name != inputValidateRe.ReplaceAllString(name, "") {
		return status.Errorf(codes.InvalidArgument, "package %s %q invalid. Must contain only [a-zA-Z0-9_.:-]", param, name)
	}
	return nil
}

func (s *server) Install(ctx context.Context, req *pb.InstallRequest) (*pb.InstallReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if err := validateField("name", req.Name); err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "invalid_name"))
		return nil, err
	}
	if err := validateField("version", req.Version); err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "invalid_version"))
		return nil, err
	}

	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}
	command, err := generateInstall(req)
	if err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
		return nil, err
	}

	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	// Default to not needeing a reboot unless exit code 1 is explicitly retured
	isRebootRequired := pb.RebootRequired_REBOOT_REQUIRED_UNKNOWN
	if needReboot, rErr := needRebootRunCmd(ctx); rErr == nil {
		isRebootRequired = pb.RebootRequired_REBOOT_REQUIRED_NO
		if needReboot {
			isRebootRequired = pb.RebootRequired_REBOOT_REQUIRED_YES
		}
	}

	return &pb.InstallReply{
		DebugOutput:    fmt.Sprintf("%s%s", run.Stdout.String(), run.Stderr.String()),
		RebootRequired: isRebootRequired,
	}, nil
}

func (s *server) Remove(ctx context.Context, req *pb.RemoveRequest) (*pb.RemoveReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if err := validateField("name", req.Name); err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "invalid_name"))
		return nil, err
	}
	if err := validateField("version", req.Version); err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "invalid_version"))
		return nil, err
	}

	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	command, err := generateRemove(req)
	if err != nil {
		recorder.CounterOrLog(ctx, packagesRemoveFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
		return nil, err
	}

	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesRemoveFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}

	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesRemoveFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	return &pb.RemoveReply{
		DebugOutput: fmt.Sprintf("%s%s", run.Stdout.String(), run.Stderr.String()),
	}, nil
}

// Nevra is of the form n-e:v-r.a (where n is optional since e can be 0 for no epoch).
var nevraRe = regexp.MustCompile(`^([^-]+-)?[^:]+:[^-]+-[^\.]+\..+$`)

func (s *server) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if err := validateField("name", req.Name); err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "invalid_name"))
		return nil, err
	}
	if err := validateField("old_version", req.OldVersion); err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "invalid_old_version"))
		return nil, err
	}
	if err := validateField("new_version", req.NewVersion); err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "invalid_new_version"))
		return nil, err
	}

	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	// Update doesn't require nevra but we do so validate each version is nevra.
	if !nevraRe.MatchString(req.OldVersion) {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "invalid_old_version"))
		return nil, status.Errorf(codes.Internal, "old_version %q not in nevra format (n-e:v-r.a)", req.OldVersion)
	}
	if !nevraRe.MatchString(req.NewVersion) {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "invalid_new_version"))
		return nil, status.Errorf(codes.Internal, "new_version %q not in nevra format (n-e:v-r.a)", req.NewVersion)
	}

	// We can generate both commands since errors duplicate here.
	validateCommand, valErr := generateValidate(req)
	updateCommand, upErr := generateUpdate(req)
	if valErr != nil || upErr != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
		return nil, fmt.Errorf("%v %v", valErr, upErr)
	}

	// First need to validate the old version is what we expect.
	run, err := util.RunCommand(ctx, validateCommand[0], validateCommand[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "package %s at version %s doesn't appear to be installed.\nstdout:\n%s\nstderr:\n%s", req.Name, req.OldVersion, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	// A 0 return means we're ok to proceed.
	run, err = util.RunCommand(ctx, updateCommand[0], updateCommand[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	return &pb.UpdateReply{
		DebugOutput: fmt.Sprintf("%s%s", run.Stdout.String(), run.Stderr.String()),
	}, nil
}

func parseListInstallOutput(p pb.PackageSystem, r io.Reader) (*pb.ListInstalledReply, error) {
	parsers := map[pb.PackageSystem]func(r io.Reader) (*pb.ListInstalledReply, error){
		pb.PackageSystem_PACKAGE_SYSTEM_YUM: parseYumListInstallOutput,
	}
	parser, ok := parsers[p]
	if !ok {
		return nil, status.Errorf(codes.Internal, "can't find parser for list install output for package system %d", p)
	}
	return parser(r)
}

func parseYumListInstallOutput(r io.Reader) (*pb.ListInstalledReply, error) {
	scanner := bufio.NewScanner(r)

	reply := &pb.ListInstalledReply{}
	started := false

	for scanner.Scan() {
		text := scanner.Text()

		// Skip lines until we find the header line. Everything after this is a package.
		if !started {
			if strings.HasPrefix(text, "Installed Packages") {
				started = true
			}
			continue
		}

		// All package lines look like:
		//
		// PACKAGE_NAME  PACKAGE_VERSION  REPO
		//
		// With no spaces (i.e. 3 fields)
		fields := strings.Fields(text)

		switch len(fields) {
		case 3:
			// Nothing to do as this is expected.
		case 2, 1:
			// 2: Sometime the version name is so long it continues onto the next line.
			// 1: Sometime the package name is so long it continues onto the next line.
			if !scanner.Scan() {
				return nil, status.Errorf(codes.Internal, "invalid input line. Expecting 3 fields and got %q and no continuation line", text)
			}
			text2 := scanner.Text()
			remaining := strings.Fields(text2)
			if len(remaining) != 3-len(fields) {
				return nil, status.Errorf(codes.Internal, "invalid input line. Expecting 3 fields and got %q and then %q on next line", text, text2)
			}
			// Now setup fields so below has everything where we expect.
			fields = append(fields, remaining...)
		default:
			// Anything else? No idea.
			return nil, status.Errorf(codes.Internal, "invalid input line. Expecting 3 fields and got %q which is invalid", text)
		}

		reply.Packages = append(reply.Packages, &pb.PackageInfo{
			Name:    fields[0],
			Version: fields[1],
			Repo:    fields[2],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "parsing error:\n%v", err)
	}

	return reply, nil
}

func (s *server) ListInstalled(ctx context.Context, req *pb.ListInstalledRequest) (*pb.ListInstalledReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	command, err := generateListInstalled(req.PackageSystem)
	if err != nil {
		recorder.CounterOrLog(ctx, packagesListInstalledFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
		return nil, err
	}

	// This may return output to stderr if the lock is held and we wait. That's ok.
	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesListInstalledFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesListInstalledFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	return parseListInstallOutput(req.PackageSystem, run.Stdout)
}

func parseRepoQueryOutput(p *pb.SearchRequest, r io.Reader) ([]string, error) {
	parsers := map[pb.PackageSystem]func(p *pb.SearchRequest, r io.Reader) ([]string, error){
		pb.PackageSystem_PACKAGE_SYSTEM_YUM: parseYumRepoQueryOutput,
	}
	parser, ok := parsers[p.PackageSystem]
	if !ok {
		return nil, status.Errorf(codes.Internal, "can't find parser for repoquery output for package system %d", p.PackageSystem)
	}
	return parser(p, r)
}

func parseYumRepoQueryOutput(p *pb.SearchRequest, r io.Reader) ([]string, error) {
	// The output of repoquery should just be package info
	// Split the output into lines
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error reading from io.Reader:")
	}
	outputLines := strings.Split(string(data), "\n")
	// Remove the leading and trailing white space
	for i, line := range outputLines {
		outputLines[i] = strings.TrimSpace(line)
	}

	return outputLines, nil
}

func (s *server) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	// check name is set
	if len(req.Name) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "package name must be filled in")
	}

	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	runRepoqueryCommand := func(command []string) ([]string, error) {
		// This may return output to stderr if the lock is held and we wait. That's ok.
		run, err := util.RunCommand(ctx, command[0], command[1:])
		if err != nil {
			recorder.CounterOrLog(ctx, packagesSearchFailureCounter, 1, attribute.String("reason", "run_err"))
			return nil, err
		}
		if err := run.Error; run.ExitCode != 0 || err != nil {
			recorder.CounterOrLog(ctx, packagesSearchFailureCounter, 1, attribute.String("reason", "run_err"))
			return nil, status.Errorf(codes.Internal, "error from running - %v\nstdout:\n%s\nstderr:\n%s", err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
		}
		packages, err := parseRepoQueryOutput(req, run.Stdout)
		if err != nil {
			recorder.CounterOrLog(ctx, packagesSearchFailureCounter, 1, attribute.String("reason", "parse_err"))
			return nil, err
		}
		return packages, nil
	}

	reply := &pb.SearchReply{}
	if req.Installed {
		command, err := generateSearch(req, true)
		if err != nil {
			recorder.CounterOrLog(ctx, packagesSearchFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
			return nil, err
		}

		packages, err := runRepoqueryCommand(command)
		if err != nil {
			return nil, err
		}
		reply.InstalledPackage = packages[0]
	}
	if req.Latest {
		command, err := generateSearch(req, false)
		if err != nil {
			recorder.CounterOrLog(ctx, packagesSearchFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
			return nil, err
		}

		packages, err := runRepoqueryCommand(command)
		if err != nil {
			return nil, err
		}
		reply.LatestPackage = packages[0]
	}
	return reply, nil
}

func parseRepoListOutput(p pb.PackageSystem, r io.Reader) (*pb.RepoListReply, error) {
	parsers := map[pb.PackageSystem]func(r io.Reader) (*pb.RepoListReply, error){
		pb.PackageSystem_PACKAGE_SYSTEM_YUM: parseYumRepoListOutput,
	}
	parser, ok := parsers[p]
	if !ok {
		return nil, status.Errorf(codes.Internal, "can't find parser for repo list output for package system %d", p)
	}
	return parser(r)
}

func parseYumRepoListOutput(r io.Reader) (*pb.RepoListReply, error) {
	scanner := bufio.NewScanner(r)

	reply := &pb.RepoListReply{}
	out := &pb.Repo{}
	numEntries := 0

	for scanner.Scan() {
		// This is a fixed column output so we can check things at specific offets.
		// i.e. lines look like this for a given entry:
		//
		// Repo-id      : updates-source/7
		// ...
		// Repo-filename: /etc/yum.repos.d/CentOS-Sources.repo
		text := scanner.Text()
		fields := strings.Fields(text)

		const MinLine = len("Repo-filename: ")

		// Ignore anything which isn't long enough or doesn't start with "Repo-"
		if len(text) < MinLine || !strings.HasPrefix(text, "Repo-") {
			continue
		}

		switch fields[0] {
		case "Repo-id":
			// This always starts a new entry.

			// If we've already processed one then we're starting a new entry
			// so tidy up and append.
			if numEntries > 0 {
				reply.Repos = append(reply.Repos, out)
				out = &pb.Repo{}
			}

			numEntries++
			// In case the repo id has spaces in it.
			out.Id = strings.Join(fields[2:], " ")
		case "Repo-name":
			// In case the repo name has spaces in it.
			out.Name = strings.Join(fields[2:], " ")
		case "Repo-status":
			switch fields[2] {
			case "disabled":
				out.Status = pb.RepoStatus_REPO_STATUS_DISABLED
			case "enabled":
				out.Status = pb.RepoStatus_REPO_STATUS_ENABLED
			default:
				out.Status = pb.RepoStatus_REPO_STATUS_UNKNOWN
			}
		case "Repo-baseurl":
			// In case the repo url has spaces in it.
			out.Url = strings.Join(fields[2:], " ")
		case "Repo-filename:":
			// In case the repo filename has spaces in it.
			// NOTE: The value part is one field less than above
			//       because there's no space before the : here.
			out.Filename = strings.Join(fields[1:], " ")
		}
	}

	// Append last one
	reply.Repos = append(reply.Repos, out)

	if err := scanner.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "parsing error:\n%v", err)
	}

	return reply, nil
}

func (s *server) RepoList(ctx context.Context, req *pb.RepoListRequest) (*pb.RepoListReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	command, err := generateRepoList(req.PackageSystem)
	if err != nil {
		recorder.CounterOrLog(ctx, packagesRepoListFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
		return nil, err
	}

	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesRepoListFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesRepoListFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error from running %q: %v\nstdout:\n%s\nstderr:\n%s", command, err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}

	return parseRepoListOutput(req.PackageSystem, run.Stdout)
}

func (s *server) Cleanup(ctx context.Context, req *pb.CleanupRequest) (*pb.CleanupResponse, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	command, err := generateCleanup(req.PackageSystem)
	if err != nil {
		recorder.CounterOrLog(ctx, packagesCleanupFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
		return nil, err
	}

	run, err := util.RunCommand(ctx, command[0], command[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesCleanupFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, err
	}
	if err := run.Error; run.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesCleanupFailureCounter, 1, attribute.String("reason", "run_err"))
		return nil, status.Errorf(codes.Internal, "error from running %q: %v\nstdout:\n%s\nstderr:\n%s", command, err, util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
	}
	return &pb.CleanupResponse{
		DebugOutput: fmt.Sprintf("%s%s", run.Stdout.String(), run.Stderr.String()),
	}, nil
}

// Install is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterPackagesServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
