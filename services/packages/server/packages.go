package server

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative packages.proto

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/packages"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Internal helper to generate the command list. The map must contain the enum.
func genCmd(p pb.PackageSystem, m map[pb.PackageSystem][]string) ([]string, error) {
	var out []string
	switch p {
	case pb.PackageSystem_PACKAGE_SYSTEM_YUM:
		out = append(out, *yumBin)
		out = append(out, m[p]...)
	default:
		return nil, status.Errorf(codes.Unimplemented, "no support for package system enum %d", p)
	}
	return out, nil
}

// Optionally add the repo arg and then append the full package name to the list.
func addRepoAndPackage(out []string, p pb.PackageSystem, name string, version string, repo string) []string {
	if repo != "" && p == pb.PackageSystem_PACKAGE_SYSTEM_YUM {
		out = append(out, fmt.Sprintf("--enablerepo=%s", repo))
	}
	// Tack the fully qualfied package name on. This assumes any vetting of args has already been done.
	out = append(out, fmt.Sprintf("%s-%s", name, version))
	return out
}

var (
	inputValidateRe = regexp.MustCompile("[^a-zA-Z0-9_.-]+")

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
		return addRepoAndPackage(out, p.PackageSystem, p.Name, p.Version, p.Repo), nil
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
		return addRepoAndPackage(out, p.PackageSystem, p.Name, p.OldVersion, ""), nil
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
		return addRepoAndPackage(out, p.PackageSystem, p.Name, p.NewVersion, p.Repo), nil
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

	generateRepoList = func(p pb.PackageSystem) ([]string, error) {
		repoOpts := map[pb.PackageSystem][]string{
			pb.PackageSystem_PACKAGE_SYSTEM_YUM: {
				"repoinfo",
				"all",
			},
		}
		return genCmd(p, repoOpts)
	}
)

// The maximum we should allow stdout or stderr to be when sending back in an error string.
// grpc has limits on how large a returned error can be (generally 4-8k depending on language).
const MAX_BUF = 1024

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
		return status.Errorf(codes.InvalidArgument, "package %s %q invalid. Must contain only [a-zA-Z0-9_-.]", param, name)
	}
	return nil
}

func (s *server) Install(ctx context.Context, req *pb.InstallRequest) (*pb.InstallReply, error) {
	if err := validateField("name", req.Name); err != nil {
		return nil, err
	}
	if err := validateField("version", req.Version); err != nil {
		return nil, err
	}

	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}
	command, err := generateInstall(req)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// These probably should be streaming through a go-routine to rate limit what we
	// can buffer. In practice output tends to be in the low K range size wise.
	var errBuf, outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Errorf(codes.Internal, "can't start %s: %v", cmd.String(), err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "error from running %s: %v", cmd.String(), err)
	}

	// This should never return stderr output. If they do something is off.
	if len(errBuf.String()) != 0 {
		if outBuf.Len() > MAX_BUF {
			outBuf.Truncate(MAX_BUF)
		}
		if errBuf.Len() > MAX_BUF {
			errBuf.Truncate(MAX_BUF)
		}
		return nil, status.Errorf(codes.Internal, "spurious output to stderr running %s\nStdout: %s\nStderr: %s", command[0], outBuf.String(), errBuf.String())
	}
	return &pb.InstallReply{
		DebugOutput: outBuf.String(),
	}, nil
}

func (s *server) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateReply, error) {
	if err := validateField("name", req.Name); err != nil {
		return nil, err
	}
	if err := validateField("old_version", req.OldVersion); err != nil {
		return nil, err
	}
	if err := validateField("new_version", req.NewVersion); err != nil {
		return nil, err
	}

	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	// First need to validate the old version is what we expect.
	command, err := generateValidate(req)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// These probably should be streaming through a go-routine to rate limit what we
	// can buffer. In practice output tends to be in the low K range size wise.
	var errBuf, outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return nil, status.Errorf(codes.Internal, "can't start validate %s: %v", cmd.String(), err)
	}
	if err := cmd.Wait(); err != nil {
		if errBuf.Len() > MAX_BUF {
			errBuf.Truncate(MAX_BUF)
		}
		return nil, status.Errorf(codes.Internal, "package %s at version %s doesn't appear to be installed.\nStderr:\n%s", req.Name, req.OldVersion, errBuf.String())
	}

	// A 0 return means we're ok to proceed.
	command, err = generateUpdate(req)
	if err != nil {
		return nil, err
	}
	errBuf.Reset()
	outBuf.Reset()

	cmd = exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return nil, status.Errorf(codes.Internal, "can't start %s: %v", cmd.String(), err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "error from running %s: %v", cmd.String(), err)
	}

	// This should never return stderr output. If they do something is off.
	if len(errBuf.String()) != 0 {
		if outBuf.Len() > MAX_BUF {
			outBuf.Truncate(MAX_BUF)
		}
		if errBuf.Len() > MAX_BUF {
			errBuf.Truncate(MAX_BUF)
		}
		return nil, status.Errorf(codes.Internal, "spurious output to stderr running %s\nStdout: %s\nStderr: %s", command[0], outBuf.String(), errBuf.String())
	}
	return &pb.UpdateReply{
		DebugOutput: outBuf.String(),
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
		if len(fields) != 3 {
			return nil, status.Errorf(codes.Internal, "invalid input line. Expecting 3 fields and got %q", text)
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
	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	command, err := generateListInstalled(req.PackageSystem)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// These probably should be streaming through a go-routine to rate limit what we
	// can buffer. In practice output tends to be in the low K range size wise.
	var errBuf, outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Errorf(codes.Internal, "can't start %s: %v", cmd.String(), err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "error from running %s: %v", cmd.String(), err)
	}

	// This should never return stderr output. If they do something is off.
	if len(errBuf.String()) != 0 {
		if outBuf.Len() > MAX_BUF {
			outBuf.Truncate(MAX_BUF)
		}
		if errBuf.Len() > MAX_BUF {
			errBuf.Truncate(MAX_BUF)
		}
		return nil, status.Errorf(codes.Internal, "spurious output to stderr running %s: %s", cmd.String(), errBuf.String())
	}

	return parseListInstallOutput(req.PackageSystem, &outBuf)
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

		const MIN_LINE = len("Repo-filename: ")

		// Ignore anything which isn't long enough or doesn't start with "Repo-"
		if len(text) < MIN_LINE || !strings.HasPrefix(text, "Repo-") {
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
	// Unset means YUM.
	if req.PackageSystem == pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN {
		req.PackageSystem = pb.PackageSystem_PACKAGE_SYSTEM_YUM
	}

	command, err := generateRepoList(req.PackageSystem)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// These probably should be streaming through a go-routine to rate limit what we
	// can buffer. In practice output tends to be in the low K range size wise.
	var errBuf, outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, status.Errorf(codes.Internal, "can't start %s: %v", cmd.String(), err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "error from running %s: %v", cmd.String(), err)
	}

	// This should never return stderr output. If they do something is off.
	if len(errBuf.String()) != 0 {
		if outBuf.Len() > MAX_BUF {
			outBuf.Truncate(MAX_BUF)
		}
		if errBuf.Len() > MAX_BUF {
			errBuf.Truncate(MAX_BUF)
		}
		return nil, status.Errorf(codes.Internal, "spurious output to stderr running %s: %s", cmd.String(), errBuf.String())
	}

	return parseRepoListOutput(req.PackageSystem, &outBuf)

}

// Install is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterPackagesServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
