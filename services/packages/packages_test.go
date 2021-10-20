package packages

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"os"
	"testing"

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

func TestListInstalled(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := NewPackagesClient(conn)

	// Test 0: Specify a bad package system and get an error.
	//         Should work with all default setup.
	resp, err := client.ListInstalled(ctx, &ListInstalledRequest{
		PackageSystem: PackageSystem_PACKAGE_SYSTEM_YUM + 1,
	})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad package enum. Instead got %+v", resp)
	}
	t.Log(err)

	// Setup for feeding in test data for further tests.
	testdataInput := "./testdata/yum-installed.out"
	testdataInputBad := "./testdata/yum-installed-bad.out"
	testdataGolden := "./testdata/yum-installed.textproto"

	savedGenerateListInstalled := generateListInstalled
	generateListInstalled = func(PackageSystem) ([]string, error) {
		return []string{"cat", testdataInput}, nil
	}
	defer func() {
		generateRepoList = savedGenerateListInstalled
	}()

	f, err := os.Open(testdataGolden)
	if err != nil {
		t.Fatalf("Can't open testdata golden %s: %v", testdataGolden, err)
	}
	defer f.Close()

	input, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Can't read textproto data from %s: %v", testdataGolden, err)
	}

	testdata := &ListInstalledReply{}
	if err := prototext.Unmarshal(input, testdata); err != nil {
		t.Fatalf("Can't unmarshall test data %v", err)
	}

	// Be able to sort the overall entries in a response
	sortEntries := protocmp.SortRepeated(func(i *PackageInfo, j *PackageInfo) bool {
		return i.Name < j.Name && i.Version < j.Version
	})

	// Test 1: No options. Should pick yum w/o error and give back our list.
	resp, err = client.ListInstalled(ctx, &ListInstalledRequest{})
	if err != nil {
		t.Fatalf("got error for a basic package list request: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	// Test 2: Specify yum this time.
	resp, err = client.ListInstalled(ctx, &ListInstalledRequest{
		PackageSystem: PackageSystem_PACKAGE_SYSTEM_YUM,
	})
	if err != nil {
		t.Fatalf("got error for a basic package list request: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	// Test 3: Now try with bad input. Should error out.
	generateListInstalled = func(PackageSystem) ([]string, error) {
		return []string{"cat", testdataInputBad}, nil
	}
	resp, err = client.ListInstalled(ctx, &ListInstalledRequest{
		PackageSystem: PackageSystem_PACKAGE_SYSTEM_YUM,
	})
	if err == nil {
		t.Fatalf("expected error for bad input. Instead got %+v", resp)
	}
	t.Log(err)

	// Test 4: Replace with a non-existant command which should error.
	generateListInstalled = func(PackageSystem) ([]string, error) {
		return []string{"/non-existant-binary"}, nil
	}
	resp, err = client.ListInstalled(ctx, &ListInstalledRequest{})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad command. Instead got %+v", resp)
	}
	t.Log(err)

	// Test 5: A command which returns a non-zero exit.
	generateListInstalled = func(PackageSystem) ([]string, error) {
		return []string{"false"}, nil
	}
	resp, err = client.ListInstalled(ctx, &ListInstalledRequest{})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad exit code. Instead got %+v", resp)
	}
	t.Log(err)

	// Test 6: A command which emits to stderr should also fail.
	generateListInstalled = func(PackageSystem) ([]string, error) {
		return []string{"sh", "-c", "echo foo >&2"}, nil
	}
	resp, err = client.ListInstalled(ctx, &ListInstalledRequest{})
	if err == nil {
		t.Fatalf("didn't get an error as expected for stderr output. Instead got %+v", resp)
	}
	t.Log(err)
}

func TestRepoList(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := NewPackagesClient(conn)

	// Test 0: Specify a bad package system and get an error.
	//         Should work with all default setup.
	resp, err := client.RepoList(ctx, &RepoListRequest{
		PackageSystem: PackageSystem_PACKAGE_SYSTEM_YUM + 1,
	})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad package enum. Instead got %+v", resp)
	}
	t.Log(err)

	// Setup for feeding in test data for further tests.
	testdataInput := "./testdata/yum-repolist.out"
	testdataGolden := "./testdata/yum-repolist.textproto"

	savedGenerateRepoList := generateRepoList
	generateRepoList = func(PackageSystem) ([]string, error) {
		return []string{"cat", testdataInput}, nil
	}
	defer func() {
		generateRepoList = savedGenerateRepoList
	}()

	f, err := os.Open(testdataGolden)
	if err != nil {
		t.Fatalf("Can't open testdata golden %s: %v", testdataGolden, err)
	}
	defer f.Close()

	input, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Can't read textproto data from %s: %v", testdataGolden, err)
	}

	testdata := &RepoListReply{}
	if err := prototext.Unmarshal(input, testdata); err != nil {
		t.Fatalf("Can't unmarshall test data %v", err)
	}

	// Be able to sort the overall entries in a response
	sortEntries := protocmp.SortRepeated(func(i *Repo, j *Repo) bool {
		return i.Id < j.Id && i.Name < j.Name
	})

	// Test 1: No options. Should pick yum w/o error and give back our list.
	resp, err = client.RepoList(ctx, &RepoListRequest{})
	if err != nil {
		t.Fatalf("got error for a basic repo list request: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	// Test 2: Specify yum this time.
	resp, err = client.RepoList(ctx, &RepoListRequest{
		PackageSystem: PackageSystem_PACKAGE_SYSTEM_YUM,
	})
	if err != nil {
		t.Fatalf("got error for a basic repo list request: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	// Test 3: Replace with a non-existant command which should error.
	generateRepoList = func(PackageSystem) ([]string, error) {
		return []string{"/non-existant-binary"}, nil
	}
	resp, err = client.RepoList(ctx, &RepoListRequest{})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad command. Instead got %+v", resp)
	}
	t.Log(err)

	// Test 4: A command which returns a non-zero exit.
	generateRepoList = func(PackageSystem) ([]string, error) {
		return []string{"false"}, nil
	}
	resp, err = client.RepoList(ctx, &RepoListRequest{})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad exit code. Instead got %+v", resp)
	}
	t.Log(err)

	// Test 5: A command which emits to stderr should also fail.
	generateRepoList = func(PackageSystem) ([]string, error) {
		return []string{"sh", "-c", "echo foo >&2"}, nil
	}
	resp, err = client.RepoList(ctx, &RepoListRequest{})
	if err == nil {
		t.Fatalf("didn't get an error as expected for stderr output. Instead got %+v", resp)
	}
	t.Log(err)
}
