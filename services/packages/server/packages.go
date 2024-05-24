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
	"strconv"
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
		out, err := genCmd(p.PackageSystem, removeOpts)
		if err != nil {
			return nil, err
		}
		repos := &repoData{
			enable:  p.Repo,
			disable: p.DisableRepo,
		}
		return addRepoAndPackage(out, p.PackageSystem, p.Name, p.Version, repos), nil
	}

	generateValidate = func(packageSystem pb.PackageSystem, name, version string) ([]string, error) {
		validateOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"list",
				"installed",
			},
		}
		out, err := genCmd(packageSystem, validateOpts)
		if err != nil {
			return nil, err
		}
		return addRepoAndPackage(out, packageSystem, name, version, &repoData{}), nil
	}

	generateUpdate = func(p *pb.UpdateRequest) ([]string, error) {
		updateOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"update-to",
				"-y",
			},
		}
		if isOlderVersion(p.NewVersion, p.OldVersion) {
			updateOpts[pb.PackageSystem_PACKAGE_SYSTEM_YUM] = []string{"downgrade", "-y"}
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

	generateSearch = func(p *pb.SearchRequest, searchType string) ([]string, error) {
		var out []string
		switch p.PackageSystem {
		case pb.PackageSystem_PACKAGE_SYSTEM_YUM:
			out = append(out, RepoqueryBin)
			out = append(out, "--nevra")
			// repoquery will load and use any YUM plugins that are installed and enabled on system.
			out = append(out, "--plugins")
			out = append(out, p.Name)
			// add options based on search type
			switch searchType {
			case "installed":
				out = append(out, "--installed")
			case "available":
				out = append(out, "--show-duplicates")
			default:
				return nil, status.Errorf(codes.InvalidArgument, "no support for package search type %s", searchType)
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

// parseToNumbers parses a string like 14:4.99.3-2.fc38.x86_64 into a slice
// of integers like [14, 4, 99, 3, 2]. Callers should ensure that the input
// is valid.
func parseToNumbers(version string) [5]int {
	var out [5]int

	// Grab epoch
	split := strings.SplitN(version, ":", 2)
	if len(split) == 2 {
		epoch, err := strconv.Atoi(split[0])
		if err != nil {
			return out
		}
		out[0] = epoch
		version = split[1]
	}

	// Grab version numbers
	split = strings.SplitN(version, "-", 2)
	if len(split) < 2 {
		return out
	}
	for i, n := range strings.SplitN(split[0], ".", 3) {
		v, err := strconv.Atoi(n)
		if err != nil {
			return out
		}
		out[1+i] = v
	}
	remainder := split[1]

	// Grab revision
	split = strings.SplitN(remainder, ".", 2)
	if len(split) < 2 {
		return out
	}
	revision, err := strconv.Atoi(split[0])
	if err != nil {
		return out
	}
	out[4] = revision

	return out
}

// isOlderVersion returns true if "a" is an older version than "b".
// Input should be the same as version in the RPC, like
// 14:4.99.3-2.fc38.x86_64 or 3.43.1-1.centos7.arm64.
// Results are indeterminate if either string isn't a proper version.
func isOlderVersion(a, b string) bool {
	parsedA := parseToNumbers(a)
	parsedB := parseToNumbers(b)
	for i := 0; i < 5; i++ {
		if parsedA[i] < parsedB[i] {
			return true
		}
		if parsedA[i] > parsedB[i] {
			return false
		}
	}
	return false
}

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

	// Make sure we actually installed the package.
	postValidateCommand, err := generateValidate(req.PackageSystem, req.Name, req.Version)
	if err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "post_validate_err"))
		return nil, err
	}
	validation, err := util.RunCommand(ctx, postValidateCommand[0], postValidateCommand[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "post_validate_err"))
		return nil, err
	}
	if err := validation.Error; validation.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesInstallFailureCounter, 1, attribute.String("reason", "post_validate_err"))
		return nil, status.Errorf(codes.Internal, "failed to confirm that package was installed\nstdout:\n%s\nstderr:\n%s\ninstall stdout:\n%s\ninstall stderr:\n%s", util.TrimString(validation.Stdout.String()), util.TrimString(validation.Stderr.String()), util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
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

// split each part of nevra
var nevraReSplit = regexp.MustCompile(`^(.+)-([\d]*):(.+)-(.+)\.(.+)$`)

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
	validateCommand, valErr := generateValidate(req.PackageSystem, req.Name, req.OldVersion)
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

	// Make sure we actually installed the package.
	postValidateCommand, err := generateValidate(req.PackageSystem, req.Name, req.NewVersion)
	if err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "post_validate_err"))
		return nil, err
	}
	validation, err := util.RunCommand(ctx, postValidateCommand[0], postValidateCommand[1:])
	if err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "post_validate_err"))
		return nil, err
	}
	if err := validation.Error; validation.ExitCode != 0 || err != nil {
		recorder.CounterOrLog(ctx, packagesUpdateFailureCounter, 1, attribute.String("reason", "post_validate_err"))
		return nil, status.Errorf(codes.Internal, "failed to confirm that package was installed\nstdout:\n%s\nstderr:\n%s\ninstall stdout:\n%s\ninstall stderr:\n%s", util.TrimString(validation.Stdout.String()), util.TrimString(validation.Stderr.String()), util.TrimString(run.Stdout.String()), util.TrimString(run.Stderr.String()))
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

// extract epoch, version and release from string with pattern: [epoch:]version-release
var extractEVR = func(evr string) (uint64, string, string, error) {
	epoch, version, release := "0", "", ""
	// in yum repo, none of epoch, version or release will contain : or -
	// we can safely split based on these separators
	parts := strings.Split(evr, ":")
	if len(parts) == 2 {
		epoch = parts[0]
		evr = parts[1]
	}
	parts = strings.Split(evr, "-")
	version = parts[0]
	if len(parts) == 2 {
		release = parts[1]
	} else {
		return 0, "", "", status.Errorf(codes.Internal, "can't extract EVR from yum list output `-` should exist between version and release")
	}

	epochUint, err := strconv.ParseUint(epoch, 10, 32)
	if err != nil {
		return 0, "", "", status.Errorf(codes.Internal, "convert epoch from string to uint error: %s", err)
	}
	return epochUint, version, release, nil
}

// extract name, arch from pattern name.arch
var extractNA = func(na string) (string, string, error) {
	// the package name itself may contain periods, but architecture cannot, so we split them based on the last period
	dotIdx := strings.LastIndex(na, ".")
	name, arch := na[:dotIdx], na[dotIdx+1:]
	return name, arch, nil
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
			for len(fields) < 3 {
				if !scanner.Scan() {
					return nil, status.Errorf(codes.Internal, "invalid input line. Expecting 3 fields and got %q and no continuation line", fields)
				}
				text2 := scanner.Text()
				remaining := strings.Fields(text2)
				if len(remaining) > 3-len(fields) {
					return nil, status.Errorf(codes.Internal, "invalid input line. Expecting 3 fields and got %q and then %q on next line", fields, text2)
				}
				// Now setup fields so below has everything where we expect.
				fields = append(fields, remaining...)
			}
		default:
			// Anything else? No idea.
			return nil, status.Errorf(codes.Internal, "invalid input line. Expecting 3 fields and got %q which is invalid", text)
		}

		// process the PACKAGE_NAME(name.arch) and PACKAGE_VERSION([epoch:]version-release)

		name, arch, err := extractNA(fields[0])
		if err != nil {
			return nil, err
		}
		epoch, version, release, err := extractEVR(fields[1])
		if err != nil {
			return nil, err
		}

		reply.Packages = append(reply.Packages, &pb.PackageInfo{
			Name:         name,
			Epoch:        uint32(epoch),
			Version:      version,
			Release:      release,
			Architecture: arch,
			Repo:         fields[2],
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

func parseRepoQueryOutput(p *pb.SearchRequest, r io.Reader) (*pb.PackageInfoList, error) {
	parsers := map[pb.PackageSystem]func(p *pb.SearchRequest, r io.Reader) (*pb.PackageInfoList, error){
		pb.PackageSystem_PACKAGE_SYSTEM_YUM: parseYumRepoQueryOutput,
	}
	parser, ok := parsers[p.PackageSystem]
	if !ok {
		return nil, status.Errorf(codes.Internal, "can't find parser for repoquery output for package system %d", p.PackageSystem)
	}
	return parser(p, r)
}

func parseYumRepoQueryOutput(p *pb.SearchRequest, r io.Reader) (*pb.PackageInfoList, error) {
	// The output of repoquery should just be package info
	// Split the output into lines
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error reading from io.Reader:")
	}
	outputLines := strings.Split(string(data), "\n")

	packageInfoList := &pb.PackageInfoList{}
	for i, line := range outputLines {
		// Remove the leading and trailing white space
		outputLines[i] = strings.TrimSpace(line)
		// skip if output line is empty
		if len(outputLines[i]) == 0 {
			continue
		}
		// extract NEVRA of each package
		matches := nevraReSplit.FindStringSubmatch(outputLines[i])
		if len(matches) != 6 {
			return nil, fmt.Errorf("invalid NEVRA format")
		}
		epoch, err := strconv.ParseUint(matches[2], 10, 32)
		if err != nil {
			// handle error
			return nil, fmt.Errorf("unable to transfer from string to int: %s", err)
		}

		packageInfo := &pb.PackageInfo{
			Name:         matches[1],
			Epoch:        uint32(epoch),
			Version:      matches[3],
			Release:      matches[4],
			Architecture: matches[5],
		}
		packageInfoList.Packages = append(packageInfoList.Packages, packageInfo)
	}
	return packageInfoList, nil
}

func (s *server) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	// check package name is set
	if len(req.Name) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "package name must be filled in")
	}
	// current search rpc requires at lease one search type: installed or available
	if !req.Installed && !req.Available {
		return nil, status.Errorf(codes.InvalidArgument, "At lease one search type (available, installed) must be set")
	}

	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	runRepoqueryCommand := func(command []string) (*pb.PackageInfoList, error) {
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
		packageInfoList, err := parseRepoQueryOutput(req, run.Stdout)
		if err != nil {
			recorder.CounterOrLog(ctx, packagesSearchFailureCounter, 1, attribute.String("reason", "parse_err"))
			return nil, err
		}
		return packageInfoList, nil
	}

	reply := &pb.SearchReply{}
	if req.Installed {
		command, err := generateSearch(req, "installed")
		if err != nil {
			recorder.CounterOrLog(ctx, packagesSearchFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
			return nil, err
		}

		packageInfoList, err := runRepoqueryCommand(command)
		if err != nil {
			return nil, err
		}
		reply.InstalledPackages = packageInfoList

	}
	if req.Available {
		command, err := generateSearch(req, "available")
		if err != nil {
			recorder.CounterOrLog(ctx, packagesSearchFailureCounter, 1, attribute.String("reason", "generate_cmd_err"))
			return nil, err
		}

		packageInfoList, err := runRepoqueryCommand(command)
		if err != nil {
			return nil, err
		}
		reply.AvailablePackages = packageInfoList
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
