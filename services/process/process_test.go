package process

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"sort"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
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

func TestList(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedPsBin := *psBin
	*psBin = "cat"
	savedFunc := osSpecificPsFlags[runtime.GOOS]
	osSpecificPsFlags[runtime.GOOS] = func() []string {
		return []string{
			fmt.Sprintf("./testdata/%s.ps", runtime.GOOS),
		}
	}
	defer func() {
		*psBin = savedPsBin
		osSpecificPsFlags[runtime.GOOS] = savedFunc
	}()

	fn := fmt.Sprintf("./testdata/%s_testdata.textproto", runtime.GOOS)
	f, err := os.Open(fn)
	if err != nil {
		t.Fatalf("Can't open testdata %s: %v", fn, err)
	}
	defer f.Close()

	input, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Can't read textproto data from %s: %v", fn, err)
	}

	testdata := &ListReply{}
	if err := prototext.Unmarshal(input, testdata); err != nil {
		t.Fatalf("Can't unmarshall test data %v", err)
	}

	// The slices aren't necessarily sorted and proto.Equal doesn't
	// sort when comparing slices (just len and element by element).
	// So fix them up. Just reassign toSort below as responses are
	// tested.
	toSort := testdata.ProcessEntries
	less := func(i, j int) bool {
		return toSort[i].Pid < toSort[j].Pid
	}
	sort.Slice(testdata.ProcessEntries, less)

	client := NewProcessClient(conn)

	// Test 1: Ask for all processes
	resp, err := client.List(ctx, &ListRequest{})
	if err != nil {
		t.Fatalf("Unexpected error for basic list: %v", err)
	}

	toSort = resp.ProcessEntries
	sort.Slice(resp.ProcessEntries, less)

	if got, want := resp, testdata; !proto.Equal(got, want) {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v", got, want)
	}

	// Test 2: Ask for just one process (use the first one in testdata)
	testPid := testdata.ProcessEntries[0].Pid
	resp, err = client.List(ctx, &ListRequest{
		Pids: []int64{testPid},
	})
	if err != nil {
		t.Fatalf("Unexpected error for single pid input: %v", err)
	}

	// Asked for 1, that should be all we get back.
	got := &ListReply{}
	for _, t := range testdata.ProcessEntries {
		if t.Pid == testPid {
			got.ProcessEntries = append(got.ProcessEntries, t)
			break
		}
	}

	// Make sure it's what we got back
	if want := resp; !proto.Equal(got, want) {
		t.Fatalf("unexpected entry count. Want %+v, got %+v\n", want, got)
	}

	// Test 3: Ask for a non-existant pid and we should get an error.

	// If we get all the pids in testdata and add them together we
	// know it's not one in the list.
	testPid = 0
	for _, p := range testdata.ProcessEntries {
		testPid += p.Pid
	}
	resp, err = client.List(ctx, &ListRequest{
		Pids: []int64{testPid},
	})
	if err == nil {
		t.Fatalf("Expected error for invalid pid. Insteaf got %+v", resp)
	}

	// Test 4: Send some bad input in and make sure we fail (also gives some
	// coverage in places we can fail).
	badFiles := []string{
		fmt.Sprintf("./testdata/%s_bad0.ps", runtime.GOOS),
		fmt.Sprintf("./testdata/%s_bad1.ps", runtime.GOOS),
	}
	for _, bf := range badFiles {
		osSpecificPsFlags[runtime.GOOS] = func() []string {
			return []string{
				bf,
			}
		}
		resp, err := client.List(ctx, &ListRequest{})
		if err == nil {
			t.Fatalf("Expected error for test file %s but got none. Instead got %+v", bf, resp)
		}
		t.Logf("Expected error: %v received", err)
	}

	// Test 5: Break the command which means we should error out.

	// Either this is where false is or nothing is there. Either way will error.
	*psBin = "/usr/bin/false"
	resp, err = client.List(ctx, &ListRequest{})
	if err == nil {
		t.Fatalf("Expected error for invalid pid. Insteaf got %+v", resp)
	}
}
