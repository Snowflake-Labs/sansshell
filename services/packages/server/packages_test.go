package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/packages"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
	conn    *grpc.ClientConn
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	lfs := &server{}
	lfs.Register(s)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

func TestInstall(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewPackagesClient(conn)

	testdataInput := "This is output we expect to see\n\nMore output\n"
	savedGenerateInstall := generateInstall
	var cmdLine string
	generateInstall = func(i *pb.InstallRequest) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateInstall(i)
		if err != nil {
			return nil, err
		}
		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "echo"), "-n", testdataInput}, nil
	}
	defer func() {
		generateInstall = savedGenerateInstall
	}()

	// Test 0: Bunch of permutations for invalid input.
	for _, test := range []struct {
		name string
		req  *pb.InstallRequest
	}{
		{
			name: "bad package system",
			req: &pb.InstallRequest{
				PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM + 1,
				Name:          "package",
				Version:       "1.2.3",
			},
		},
		{
			name: "no name given",
			req: &pb.InstallRequest{
				Version: "1.2.3",
			},
		},
		{
			name: "no version given",
			req: &pb.InstallRequest{
				Name: "package",
			},
		},
		{
			name: "bad name - starts with a dash",
			req: &pb.InstallRequest{
				Name:    "-package",
				Version: "1.2.3",
			},
		},
		{
			name: "bad version - starts with a dash",
			req: &pb.InstallRequest{
				Name:    "package",
				Version: "-1.2.3",
			},
		},
		{
			name: "invalid characters in name",
			req: &pb.InstallRequest{
				Name:    "package && rm -rf /",
				Version: "1.2.3",
			},
		},
		{
			name: "invalid characters in version",
			req: &pb.InstallRequest{
				Name:    "package",
				Version: "1.2.3 && rm -rf /",
			},
		},
	} {
		resp, err := client.Install(ctx, test.req)
		if err == nil {
			t.Fatalf("didn't get an error as expected for a %s. Instead got %+v", test.name, resp)
		}
		t.Logf("%s: %v", test.name, err)
	}

	req := &pb.InstallRequest{
		Name:    "package",
		Version: "1.2.3",
		Repo:    "somerepo",
	}

	// Test 1: A clean install. Validate we got expected output back.

	// This is assuming yum based installs for testing command builder.
	wantCmdLine := fmt.Sprintf("%s install-nevra -y --enablerepo=somerepo package-1.2.3", *yumBin)

	resp, err := client.Install(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error from a clean install request: %v", err)
	}
	if got, want := resp.DebugOutput, testdataInput; got != want {
		t.Fatalf("Output from clean install differs. Got:\n%q\nWant:\n%q", got, want)
	}
	if got, want := cmdLine, wantCmdLine; got != want {
		t.Fatalf("command lines differ. Got %q Want %q", got, want)
	}
	t.Logf("clean install response: %+v", resp)

	// Test 2: Permutations on bad commands/output.
	for _, test := range []struct {
		name     string
		generate func(*pb.InstallRequest) ([]string, error)
	}{
		{
			name: "bad command",
			generate: func(*pb.InstallRequest) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
		},
		{
			name: "bad exit code",
			generate: func(*pb.InstallRequest) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
		},
	} {
		generateInstall = test.generate
		resp, err := client.Install(ctx, req)
		if err == nil {
			t.Fatalf("didn't get expected error for %s Got %+v", test.name, resp)
		}
		t.Log(err)

	}
}

func TestUpdate(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewPackagesClient(conn)

	testdataInput := "This is output we expect to see\n\nMore output\n"
	savedGenerateValidate := generateValidate
	savedGenerateUpdate := generateUpdate
	var cmdLine, validateCmdLine string
	generateValidate = func(u *pb.UpdateRequest) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateValidate(u)
		if err != nil {
			return nil, err
		}
		validateCmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "echo"), "-n", testdataInput}, nil
	}
	generateUpdate = func(u *pb.UpdateRequest) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateUpdate(u)
		if err != nil {
			return nil, err
		}
		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "echo"), "-n", testdataInput}, nil
	}
	defer func() {
		generateValidate = savedGenerateValidate
		generateUpdate = savedGenerateUpdate
	}()

	// Test 0: Bunch of permutations for invalid input.
	for _, test := range []struct {
		name string
		req  *pb.UpdateRequest
	}{
		{
			name: "bad package system",
			req: &pb.UpdateRequest{
				PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM + 1,
				Name:          "package",
				OldVersion:    "1.2.3",
				NewVersion:    "4.5.6",
			},
		},
		{
			name: "no name given",
			req: &pb.UpdateRequest{
				OldVersion: "1.2.3",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "no old version given",
			req: &pb.UpdateRequest{
				Name:       "package",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "no new version given",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "1.2.3",
			},
		},
		{
			name: "bad name - starts with a dash",
			req: &pb.UpdateRequest{
				Name:       "-package",
				OldVersion: "1.2.3",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "bad old version - starts with a dash",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "-1.2.3",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "bad new version - starts with a dash",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "1.2.3",
				NewVersion: "-4.5.6",
			},
		},
		{
			name: "invalid characters in name",
			req: &pb.UpdateRequest{
				Name:       "package && rm -rf /",
				OldVersion: "1.2.3",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "invalid characters in old version",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "1.2.3 && rm -rf /",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "invalid characters in new version",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "1.2.3",
				NewVersion: "4.5.6 && rm -rf /",
			},
		},
	} {
		resp, err := client.Update(ctx, test.req)
		if err == nil {
			t.Fatalf("didn't get an error as expected for a %s. Instead got %+v", test.name, resp)
		}
		t.Logf("%s: %v", test.name, err)
	}

	req := &pb.UpdateRequest{
		Name:       "package",
		OldVersion: "0:1-1.2.3",
		NewVersion: "0:1-4.5.6",
		Repo:       "somerepo",
	}

	// Test 1: A clean install. Validate we got expected output back.

	// This is assuming yum based installs for testing command builder.
	wantValidateCmdLine := fmt.Sprintf("%s list installed package-0:1-1.2.3", *yumBin)
	wantCmdLine := fmt.Sprintf("%s update-to -y --enablerepo=somerepo package-0:1-4.5.6", *yumBin)

	resp, err := client.Update(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error from a clean update request: %v", err)
	}
	if got, want := resp.DebugOutput, testdataInput; got != want {
		t.Fatalf("Output from clean update differs. Got:\n%q\nWant:\n%q", got, want)
	}
	if got, want := validateCmdLine, wantValidateCmdLine; got != want {
		t.Fatalf("validate command lines differ. Got %q Want %q", got, want)
	}
	if got, want := cmdLine, wantCmdLine; got != want {
		t.Fatalf("command lines differ. Got %q Want %q", got, want)
	}
	t.Logf("clean install response: %+v", resp)

	// Test 2: Permutations on bad commands/output.
	for _, test := range []struct {
		name     string
		generate func(*pb.UpdateRequest) ([]string, error)
	}{
		{
			name: "bad command",
			generate: func(*pb.UpdateRequest) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
		},
		{
			name: "bad exit code",
			generate: func(*pb.UpdateRequest) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
		},
	} {
		generateUpdate = test.generate
		resp, err := client.Update(ctx, req)
		if err == nil {
			t.Fatalf("didn't get expected error for %s Got %+v", test.name, resp)
		}
		t.Log(err)
	}
}
func TestListInstalled(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewPackagesClient(conn)

	// Setup for feeding in test data for further tests.
	testdataInput := "./testdata/yum-installed.out"
	testdataInputBad := "./testdata/yum-installed-bad.out"
	testdataGolden := "./testdata/yum-installed.textproto"

	savedGenerateListInstalled := generateListInstalled
	var cmdLine string
	generateListInstalled = func(p pb.PackageSystem) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateListInstalled(p)
		if err != nil {
			return nil, err
		}
		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "cat"), testdataInput}, nil
	}
	defer func() {
		generateListInstalled = savedGenerateListInstalled
	}()

	input, err := os.ReadFile(testdataGolden)
	if err != nil {
		t.Fatalf("Can't read testdata golden  from %s: %v", testdataGolden, err)
	}

	testdata := &pb.ListInstalledReply{}
	if err := prototext.Unmarshal(input, testdata); err != nil {
		t.Fatalf("Can't unmarshall test data %v", err)
	}

	// Be able to sort the overall entries in a response
	sortEntries := protocmp.SortRepeated(func(i *pb.PackageInfo, j *pb.PackageInfo) bool {
		return i.Name < j.Name && i.Version < j.Version
	})

	// Test 0: Specify a bad package system and get an error.
	resp, err := client.ListInstalled(ctx, &pb.ListInstalledRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM + 1,
	})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad package enum. Instead got %+v", resp)
	}
	t.Log(err)

	// Test 1: No options. Should pick yum w/o error and give back our list.

	// This is assuming yum based installs for testing command builder.
	wantCmdLine := fmt.Sprintf("%s list installed", *yumBin)

	resp, err = client.ListInstalled(ctx, &pb.ListInstalledRequest{})
	if err != nil {
		t.Fatalf("got error for a basic package list request: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	if got, want := cmdLine, wantCmdLine; got != want {
		t.Fatalf("command lines differ. Got %q Want %q", got, want)
	}

	// Test 2: Specify yum this time.
	resp, err = client.ListInstalled(ctx, &pb.ListInstalledRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
	})
	if err != nil {
		t.Fatalf("got error for a basic package list request: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	// Test 3: Now try with bad input. Should error out.
	generateListInstalled = func(pb.PackageSystem) ([]string, error) {
		return []string{testutil.ResolvePath(t, "cat"), testdataInputBad}, nil
	}
	resp, err = client.ListInstalled(ctx, &pb.ListInstalledRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
	})
	if err == nil {
		t.Fatalf("expected error for bad input. Instead got %+v", resp)
	}
	t.Log(err)

	// Test 4: Permutations of bad commands/exit codes, stderr output.
	for _, test := range []struct {
		name     string
		generate func(pb.PackageSystem) ([]string, error)
	}{
		{
			name: "non-existant binary",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
		},
		{
			name: "non-zero exit",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
		},
		{
			name: "stderr output",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{testutil.ResolvePath(t, "sh"), "-c", "echo foo >&2"}, nil
			},
		},
	} {
		generateListInstalled = test.generate
		resp, err = client.ListInstalled(ctx, &pb.ListInstalledRequest{})
		if err == nil {
			t.Fatalf("didn't get an error as expected for %s. Instead got %+v", test.name, resp)
		}
		t.Log(err)
	}
}

func TestRepoList(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewPackagesClient(conn)

	// Setup for feeding in test data for further tests.
	testdataInput := "./testdata/yum-repolist.out"
	testdataGolden := "./testdata/yum-repolist.textproto"

	savedGenerateRepoList := generateRepoList
	var cmdLine string
	generateRepoList = func(p pb.PackageSystem) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateRepoList(p)
		if err != nil {
			return nil, err
		}
		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "cat"), testdataInput}, nil
	}
	defer func() {
		generateRepoList = savedGenerateRepoList
	}()

	input, err := os.ReadFile(testdataGolden)
	if err != nil {
		t.Fatalf("Can't read testdata golde from %s: %v", testdataGolden, err)
	}

	testdata := &pb.RepoListReply{}
	if err := prototext.Unmarshal(input, testdata); err != nil {
		t.Fatalf("Can't unmarshall test data %v", err)
	}

	// Be able to sort the overall entries in a response
	sortEntries := protocmp.SortRepeated(func(i *pb.Repo, j *pb.Repo) bool {
		return i.Id < j.Id && i.Name < j.Name
	})

	// Test 0: Specify a bad package system and get an error.
	resp, err := client.RepoList(ctx, &pb.RepoListRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM + 1,
	})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad package enum. Instead got %+v", resp)
	}
	t.Log(err)

	// Test 1: No options. Should pick yum w/o error and give back our list.

	// This is assuming yum based installs for testing command builder.
	wantCmdLine := fmt.Sprintf("%s repoinfo all", *yumBin)

	resp, err = client.RepoList(ctx, &pb.RepoListRequest{})
	if err != nil {
		t.Fatalf("got error for a basic repo list request: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	if got, want := cmdLine, wantCmdLine; got != want {
		t.Fatalf("command lines differ. Got %q Want %q", got, want)
	}
	// Test 2: Specify yum this time.
	resp, err = client.RepoList(ctx, &pb.RepoListRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
	})
	if err != nil {
		t.Fatalf("got error for a basic repo list request: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	// Test 3: Permutations of bad commands/exit codes, stderr output.
	for _, test := range []struct {
		name     string
		generate func(pb.PackageSystem) ([]string, error)
	}{
		{
			name: "non-existant binary",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
		},
		{
			name: "non-zero exit",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
		},
	} {
		generateRepoList = test.generate
		resp, err = client.RepoList(ctx, &pb.RepoListRequest{})
		if err == nil {
			t.Fatalf("didn't get an error as expected for %s. Instead got %+v", test.name, resp)
		}
		t.Log(err)
	}
}
