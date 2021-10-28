package server

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/process"
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

	testdataJstack     = "./testdata/jstack.txt"
	testdataJstackBad  = "./testdata/jstack-bad.txt"
	testdataJstackBad1 = "./testdata/jstack-bad1.txt"
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
	// We're on a platform which doesn't support this so we can't test.
	if *psBin == "" {
		t.Skip("OS not supported")
	}

	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewProcessClient(conn)

	// Ask for all processes
	resp, err := client.List(ctx, &pb.ListRequest{})
	if err != nil {
		t.Fatalf("Unexpected error for basic list: %v", err)
	}

	if len(resp.ProcessEntries) == 0 {
		t.Errorf("Returned ps list is empty?")
	}

	// Pid 1 should be stable on all unix.
	pid := int64(1)
	resp, err = client.List(ctx, &pb.ListRequest{
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
	// We're on a platform which doesn't support this so we can't test.
	if testdataPsTextProto == "" {
		t.Skip("OS not supported")
	}

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
	psOptions = func() []string {
		return []string{
			testdataPs,
		}
	}
	defer func() {
		*psBin = savedPsBin
		psOptions = savedFunc
	}()

	f, err := os.Open(testdataPsTextProto)
	if err != nil {
		t.Fatalf("Can't open testdata %s: %v", testdataPsTextProto, err)
	}
	defer f.Close()

	input, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Can't read textproto data from %s: %v", testdataPsTextProto, err)
	}

	testdata := &pb.ListReply{}
	if err := prototext.Unmarshal(input, testdata); err != nil {
		t.Fatalf("Can't unmarshall test data %v", err)
	}

	// Some sorting functions for protocmp below.

	// Be able to sort the overall entries in a response
	sortEntries := protocmp.SortRepeated(func(i *pb.ProcessEntry, j *pb.ProcessEntry) bool {
		return i.Pid < j.Pid
	})

	// A sorter for the repeated fields of ProcessStateCode.
	sortCodes := protocmp.SortRepeated(func(i pb.ProcessStateCode, j pb.ProcessStateCode) bool {
		return i < j
	})

	client := pb.NewProcessClient(conn)

	// Test 1: Ask for all processes
	resp, err := client.List(ctx, &pb.ListRequest{})
	if err != nil {
		t.Fatalf("Unexpected error for basic list: %v", err)
	}

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries, sortCodes); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	// Test 2: Ask for just one process (use the first one in testdata)
	testPid := testdata.ProcessEntries[0].Pid
	resp, err = client.List(ctx, &pb.ListRequest{
		Pids: []int64{testPid},
	})
	if err != nil {
		t.Fatalf("Unexpected error for single pid input: %v", err)
	}

	// Asked for 1, that should be all we get back.
	got := &pb.ListReply{}
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
	resp, err = client.List(ctx, &pb.ListRequest{
		Pids: []int64{testPid},
	})
	if err == nil {
		t.Fatalf("Expected error for invalid pid. Insteaf got %+v", resp)
	}

	// Test 4: Send some bad input in and make sure we fail (also gives some
	// coverage in places we can fail).
	for _, bf := range badFilesPs {
		psOptions = func() []string {
			return []string{
				bf,
			}
		}
		resp, err := client.List(ctx, &pb.ListRequest{})
		if err == nil {
			t.Fatalf("Expected error for test file %s but got none. Instead got %+v", bf, resp)
		}
		t.Logf("Expected error: %v received", err)
	}

	// Test 5: Break the command which means we should error out.

	// Either this is where false is or nothing is there. Either way will error.
	*psBin = "/usr/bin/false"
	resp, err = client.List(ctx, &pb.ListRequest{})
	if err == nil {
		t.Fatalf("Expected error for command returning non-zero Insteaf got %+v", resp)
	}

	// Test 6: Run an invalid command all-together.
	*psBin = "/a/non-existant/command"
	resp, err = client.List(ctx, &pb.ListRequest{})
	if err == nil {
		t.Fatalf("Expected error for invalid command. Insteaf got %+v", resp)
	}

	// Test 7: Command with stderr output.
	*psBin = "sh"
	psOptions = func() []string {
		return []string{"-c",
			"echo boo 1>&2",
		}
	}
	resp, err = client.List(ctx, &pb.ListRequest{})
	if err == nil {
		t.Fatalf("Expected error for stderr output. Insteaf got %+v", resp)
	}
}

func TestPstackNative(t *testing.T) {
	_, err := os.Stat(*pstackBin)
	if *pstackBin == "" || err != nil {
		t.Skip("OS not supported")
	}

	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewProcessClient(conn)

	// Our actual pid may not have symbols but our runner will.
	resp, err := client.GetStacks(ctx, &pb.GetStacksRequest{
		Pid: int64(os.Getppid()),
	})
	if err != nil {
		t.Fatalf("Can't get native pstack: %v", err)
	}

	// We're a go program. We have multiple threads.
	if len(resp.Stacks) <= 1 {
		t.Fatalf("Not enough threads in native response. Response: %+v", prototext.Format(resp))
	}

	found := false
	for _, stack := range resp.Stacks {
		for _, t := range stack.Stacks {
			if strings.Contains(t, "runtime.") {
				found = true
				break
			}
		}
		if found == true {
			break
		}
	}

	if !found {
		t.Fatalf("Never found a runtime stack in output? Response: %+v", prototext.Format(resp))
	}
}

func TestPstack(t *testing.T) {
	if testdataPstackNoThreads == "" || testdataPstackThreads == "" {
		t.Skip("OS not supported")
	}

	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedPstackBin := *pstackBin
	*pstackBin = "cat"
	savedPstackOptions := pstackOptions
	var testInput string
	pstackOptions = func(*pb.GetStacksRequest) []string {
		return []string{testInput}
	}
	defer func() {
		*pstackBin = savedPstackBin
		pstackOptions = savedPstackOptions
	}()

	client := pb.NewProcessClient(conn)

	goodPstackOptions := pstackOptions

	for _, test := range []struct {
		name     string
		command  string
		options  []string
		input    string
		validate string
		pid      int64
		wantErr  bool
	}{
		{
			name:     "A program with only one thread",
			input:    testdataPstackNoThreads,
			validate: testdataPstackNoThreadsTextProto,
			pid:      1,
		},
		{
			name:     "A program with many threads",
			input:    testdataPstackThreads,
			validate: testdataPstackThreadsTextProto,
			pid:      1,
		},
		{
			name:    "bad pid - zero",
			input:   testdataPstackThreads,
			wantErr: true,
		},
		{
			name:    "bad command",
			command: "/non-existant-binary",
			input:   testdataPstackThreads,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "bad command - returns error",
			command: "false",
			input:   testdataPstackThreads,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "bad command - returns stderr",
			command: "sh",
			options: []string{"-c", "echo foo >&2"},
			input:   testdataPstackThreads,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad thread data - thread",
			input:   testdataPstackThreadsBadThread,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad thread data - number",
			input:   testdataPstackThreadsBadThreadNumber,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad thread data - id",
			input:   testdataPstackThreadsBadThreadId,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad thread data - lwp",
			input:   testdataPstackThreadsBadLwp,
			pid:     1,
			wantErr: true,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			command := "cat"
			if test.command != "" {
				command = test.command
			}
			*pstackBin = command

			pstackOptions = goodPstackOptions
			testInput = test.input

			if len(test.options) > 0 {
				pstackOptions = func(req *pb.GetStacksRequest) []string {
					return test.options
				}
			}

			testdata := &pb.GetStacksReply{}
			// In general we don't need this if we expect errors.
			if test.validate != "" {
				f, err := os.Open(test.validate)
				if err != nil {
					t.Fatalf("%s: Can't open testdata %s: %v", test.name, test.validate, err)
				}
				defer f.Close()

				input, err := ioutil.ReadAll(f)
				if err != nil {
					t.Fatalf("%s: Can't read textproto data from %s: %v", test.name, test.validate, err)
				}

				if err := prototext.Unmarshal(input, testdata); err != nil {
					t.Fatalf("%s: Can't unmarshall test data %v", test.name, err)
				}
			}

			resp, err := client.GetStacks(ctx, &pb.GetStacksRequest{Pid: test.pid})
			if err != nil && !test.wantErr {
				t.Fatalf("%s: unexpected error: %v", test.name, err)
			}
			if err == nil && test.wantErr {
				t.Fatalf("%s: didn't get expected error. Response: %+v", test.name, resp)
			}

			if !test.wantErr {
				if diff := cmp.Diff(resp, testdata, protocmp.Transform()); diff != "" {
					t.Fatalf("%s: Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", test.name, resp, testdata, diff)
				}
			}
		})
	}
}

func TestJstack(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewProcessClient(conn)

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedJstackBin := *jstackBin
	savedFunc := jstackOptions
	var testInput string
	jstackOptions = func(*pb.GetJavaStacksRequest) []string {
		return []string{
			testInput,
		}
	}
	defer func() {
		*jstackBin = savedJstackBin
		jstackOptions = savedFunc
	}()

	goodJstackOptions := jstackOptions

	for _, test := range []struct {
		name     string
		command  string
		input    string
		validate string
		pid      int64
		wantErr  bool
	}{
		{
			name:     "Basic jstack output",
			input:    testdataJstack,
			validate: "./testdata/jstack.textproto",
			pid:      1,
		},
		{
			name:    "Bad pid",
			input:   testdataJstack,
			wantErr: true,
		},
		{
			name:    "No end quote",
			input:   testdataJstackBad,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad field",
			input:   testdataJstackBad1,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad command",
			input:   testdataJstack,
			pid:     1,
			command: "/non-existant-command",
			wantErr: true,
		},
		{
			name:    "Command returns error",
			input:   testdataJstack,
			pid:     1,
			command: "false",
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := "cat"
			if test.command != "" {
				command = test.command
			}
			*jstackBin = command

			jstackOptions = goodJstackOptions
			testInput = test.input

			testdata := &pb.GetJavaStacksReply{}
			if test.validate != "" {
				f, err := os.Open(test.validate)
				if err != nil {
					t.Fatalf("%s: Can't open testdata %s: %v", test.name, test.validate, err)
				}
				defer f.Close()

				input, err := ioutil.ReadAll(f)
				if err != nil {
					t.Fatalf("%s: Can't read textproto data from %s: %v", test.name, test.validate, err)
				}
				if err := prototext.Unmarshal(input, testdata); err != nil {
					t.Fatalf("%s: Can't unmarshall test data %v", test.name, err)
				}

			}

			resp, err := client.GetJavaStacks(ctx, &pb.GetJavaStacksRequest{Pid: test.pid})
			if err != nil && !test.wantErr {
				t.Fatalf("%s: unexpected error: %v", test.name, err)
			}
			if err == nil && test.wantErr {
				t.Fatalf("%s: didn't get expected error. Response: %+v", test.name, resp)
			}

			if !test.wantErr {
				if diff := cmp.Diff(resp, testdata, protocmp.Transform()); diff != "" {
					t.Fatalf("%s: Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", test.name, resp, testdata, diff)
				}
			}
		})
	}
}
