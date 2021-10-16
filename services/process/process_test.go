package process

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

func TestListNative(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := NewProcessClient(conn)

	// Ask for all processes
	resp, err := client.List(ctx, &ListRequest{})
	if err != nil {
		t.Fatalf("Unexpected error for basic list: %v", err)
	}

	if len(resp.ProcessEntries) == 0 {
		t.Errorf("Returned ps list is empty?")
	}

	// Pid 1 should be stable on all unix.
	pid := int64(1)
	resp, err = client.List(ctx, &ListRequest{
		Pids: []int64{pid},
	})

	if err != nil {
		t.Fatalf("Unexpected error for basic list: %v", err)
	}

	if len(resp.ProcessEntries) != 1 {
		t.Fatalf("Asked for a single entry and got back something else? %+v", resp)
	}

	if pid != resp.ProcessEntries[0].Pid {
		t.Fatalf("Pids don't match. Expecting %d and got back entry: %+v", pid, resp)
	}
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
	savedFunc := psOptions
	psOptions = func() ([]string, error) {
		return []string{
			testdataPs,
		}, nil
	}
	defer func() {
		*psBin = savedPsBin
		psOptions = savedFunc
	}()

	f, err := os.Open(testdataFile)
	if err != nil {
		t.Fatalf("Can't open testdata %s: %v", testdataFile, err)
	}
	defer f.Close()

	input, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Can't read textproto data from %s: %v", testdataFile, err)
	}

	testdata := &ListReply{}
	if err := prototext.Unmarshal(input, testdata); err != nil {
		t.Fatalf("Can't unmarshall test data %v", err)
	}

	// Some sorting functions for protocmp below.

	// Be able to sort the overall entries in a response
	sortEntries := protocmp.SortRepeated(func(i *ProcessEntry, j *ProcessEntry) bool {
		return i.Pid < j.Pid
	})

	// A sorter for the repeated fields of ProcessStateCode.
	sortCodes := protocmp.SortRepeated(func(i ProcessStateCode, j ProcessStateCode) bool {
		return i < j
	})

	client := NewProcessClient(conn)

	// Test 1: Ask for all processes
	resp, err := client.List(ctx, &ListRequest{})
	if err != nil {
		t.Fatalf("Unexpected error for basic list: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries, sortCodes); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
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
	if diff := cmp.Diff(got, resp, protocmp.Transform(), sortEntries, sortCodes); diff != "" {
		t.Fatalf("unexpected entry count. Want %+v, got %+v\nDiff:\n%s", resp, got, diff)
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
	for _, bf := range badFiles {
		psOptions = func() ([]string, error) {
			return []string{
				bf,
			}, nil
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
