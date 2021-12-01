package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/process"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/google/go-cmp/cmp"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
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
	testutil.FatalOnErr("failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewProcessClient(conn)

	// Ask for all processes
	resp, err := client.List(ctx, &pb.ListRequest{})
	testutil.FatalOnErr("unexpected error for basic list", err, t)

	if len(resp.ProcessEntries) == 0 {
		t.Errorf("Returned ps list is empty?")
	}

	// Pid 1 should be stable on all unix.
	pid := int64(1)
	resp, err = client.List(ctx, &pb.ListRequest{
		Pids: []int64{pid},
	})
	testutil.FatalOnErr("unexpected error for basic list", err, t)

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
	testutil.FatalOnErr("failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedPsBin := *psBin
	*psBin = testutil.ResolvePath(t, "cat")
	savedFunc := psOptions
	psOptions = func() []string {
		return []string{
			testdataPs,
		}
	}
	t.Cleanup(func() {
		*psBin = savedPsBin
		psOptions = savedFunc
	})

	input, err := os.ReadFile(testdataPsTextProto)
	testutil.FatalOnErr(fmt.Sprintf("can't open testdata %s", testdataPsTextProto), err, t)

	testdata := &pb.ListReply{}
	err = prototext.Unmarshal(input, testdata)
	testutil.FatalOnErr("can't unmarshal test data", err, t)

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
	testutil.FatalOnErr("unexpected error for basic list", err, t)

	if diff := cmp.Diff(resp, testdata, protocmp.Transform(), sortEntries, sortCodes); diff != "" {
		t.Fatalf("Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", resp, testdata, diff)
	}

	// Test 2: Ask for just one process (use the first one in testdata)
	testPid := testdata.ProcessEntries[0].Pid
	resp, err = client.List(ctx, &pb.ListRequest{
		Pids: []int64{testPid},
	})
	testutil.FatalOnErr("unexpected error for single pid input", err, t)

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

	*psBin = testutil.ResolvePath(t, "false")
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
	*psBin = testutil.ResolvePath(t, "sh")
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
	testutil.FatalOnErr("failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewProcessClient(conn)

	// Our actual pid may not have symbols but our runner will.
	resp, err := client.GetStacks(ctx, &pb.GetStacksRequest{
		Pid: int64(os.Getppid()),
	})
	testutil.FatalOnErr("can't get native pstack", err, t)

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
		t.Fatalf("pstack() : want response with stack containing 'runtime', got %v", prototext.Format(resp))
	}
}

func TestPstack(t *testing.T) {
	if testdataPstackNoThreads == "" || testdataPstackThreads == "" {
		t.Skip("OS not supported")
	}

	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedPstackBin := *pstackBin
	*pstackBin = testutil.ResolvePath(t, "cat")
	savedFunc := pstackOptions
	var testInput string
	pstackOptions = func(req *pb.GetStacksRequest) []string {
		if opts := savedFunc(req); len(opts) != 1 {
			t.Fatalf("bad pstack options. Expected 1 got %q", opts)
		}
		return []string{testInput}
	}
	t.Cleanup(func() {
		*pstackBin = savedPstackBin
		pstackOptions = savedFunc
	})

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
			command:  testutil.ResolvePath(t, "cat"),
			input:    testdataPstackNoThreads,
			validate: testdataPstackNoThreadsTextProto,
			pid:      1,
		},
		{
			name:     "A program with many threads",
			command:  testutil.ResolvePath(t, "cat"),
			input:    testdataPstackThreads,
			validate: testdataPstackThreadsTextProto,
			pid:      1,
		},
		{
			name:    "bad pid - zero",
			command: testutil.ResolvePath(t, "cat"),
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
			name:    "no command",
			input:   testdataPstackThreads,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "bad command - returns error",
			command: testutil.ResolvePath(t, "false"),
			input:   testdataPstackThreads,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "bad command - returns stderr",
			command: testutil.ResolvePath(t, "sh"),
			options: []string{"-c", "echo foo >&2"},
			input:   testdataPstackThreads,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad thread data - thread",
			command: testutil.ResolvePath(t, "cat"),
			input:   testdataPstackThreadsBadThread,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad thread data - number",
			command: testutil.ResolvePath(t, "cat"),
			input:   testdataPstackThreadsBadThreadNumber,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad thread data - id",
			command: testutil.ResolvePath(t, "cat"),
			input:   testdataPstackThreadsBadThreadId,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad thread data - lwp",
			command: testutil.ResolvePath(t, "cat"),
			input:   testdataPstackThreadsBadLwp,
			pid:     1,
			wantErr: true,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			*pstackBin = test.command

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
				input, err := os.ReadFile(test.validate)
				testutil.FatalOnErr(fmt.Sprintf("can't open testdata %s", test.validate), err, t)
				err = prototext.Unmarshal(input, testdata)
				testutil.FatalOnErr("can't unmarshal test data", err, t)
			}

			resp, err := client.GetStacks(ctx, &pb.GetStacksRequest{Pid: test.pid})
			if got, want := err != nil, test.wantErr; got != want {
				t.Fatalf("%s: unexpected error state. got %t want %t err %v", test.name, got, want, err)
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
	testutil.FatalOnErr("failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewProcessClient(conn)

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedJstackBin := *jstackBin
	savedFunc := jstackOptions
	var testInput string
	jstackOptions = func(req *pb.GetJavaStacksRequest) []string {
		if opts := savedFunc(req); len(opts) != 1 {
			t.Fatalf("bad pstack options. Expected 1 got %q", opts)
		}
		return []string{
			testInput,
		}
	}
	t.Cleanup(func() {
		*jstackBin = savedJstackBin
		jstackOptions = savedFunc
	})

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
			command:  testutil.ResolvePath(t, "cat"),
			input:    testdataJstack,
			validate: "./testdata/jstack.textproto",
			pid:      1,
		},
		{
			name:    "No command",
			input:   testdataJstack,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad pid",
			command: testutil.ResolvePath(t, "cat"),
			input:   testdataJstack,
			wantErr: true,
		},
		{
			name:    "No end quote",
			command: testutil.ResolvePath(t, "cat"),
			input:   testdataJstackBad,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad field",
			command: testutil.ResolvePath(t, "cat"),
			input:   testdataJstackBad1,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad command",
			command: "/non-existant-command",
			input:   testdataJstack,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Bad command - path",
			command: "foo",
			input:   testdataJstack,
			pid:     1,
			wantErr: true,
		},
		{
			name:    "Command returns error",
			command: testutil.ResolvePath(t, "false"),
			input:   testdataJstack,
			pid:     1,
			wantErr: true,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			*jstackBin = test.command

			jstackOptions = goodJstackOptions
			testInput = test.input

			testdata := &pb.GetJavaStacksReply{}
			if test.validate != "" {
				input, err := os.ReadFile(test.validate)
				testutil.FatalOnErr(fmt.Sprintf("can't open testdata %s", test.validate), err, t)
				err = prototext.Unmarshal(input, testdata)
				testutil.FatalOnErr("can't unmarshal test data", err, t)
			}

			resp, err := client.GetJavaStacks(ctx, &pb.GetJavaStacksRequest{Pid: test.pid})
			if got, want := err != nil, test.wantErr; got != want {
				t.Fatalf("%s: unexpected error state. got %t want %t err %v", test.name, got, want, err)
			}

			if !test.wantErr {
				if diff := cmp.Diff(resp, testdata, protocmp.Transform()); diff != "" {
					t.Fatalf("%s: Responses differ.\nGot\n%+v\n\nWant\n%+v\nDiff:\n%s", test.name, resp, testdata, diff)
				}
			}
		})
	}
}

func TestMemoryDump(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewProcessClient(conn)

	// Setup for tests where we use cat and pre-canned data
	// to submit into the server.
	savedGcoreBin := *gcoreBin
	savedGcoreFunc := gcoreOptionsAndLocation
	savedJmapBin := *jmapBin
	savedJmapFunc := jmapOptionsAndLocation

	// The default one assumes we're just echoing file contents with cat.
	var testInput string
	gcoreOptionsAndLocation = func(req *pb.GetMemoryDumpRequest) ([]string, string, error) {
		opts, file, err := savedGcoreFunc(req)
		testutil.FatalOnErr(fmt.Sprintf("error from gcoreOptionsAndLocation for req %+v", req), err, t)
		if len(opts) == 0 {
			t.Fatalf("didn't get any options back from gcoreOptionsAndLocation for req: %+v", req)
		}
		defer os.RemoveAll(filepath.Dir(file)) // clean up

		return []string{
			testInput,
		}, testInput, nil
	}
	jmapOptionsAndLocation = func(req *pb.GetMemoryDumpRequest) ([]string, string, error) {
		opts, file, err := savedJmapFunc(req)
		testutil.FatalOnErr(fmt.Sprintf("error from jmapOptionsAndLocation for req %+v", req), err, t)
		if len(opts) == 0 {
			t.Fatalf("didn't get any options back from jmapOptionsAndLocation for req: %+v", req)
		}
		defer os.RemoveAll(filepath.Dir(file)) // clean up

		return []string{
			testInput,
		}, testInput, nil
	}
	t.Cleanup(func() {
		*gcoreBin = savedGcoreBin
		*jmapBin = savedJmapBin
		gcoreOptionsAndLocation = savedGcoreFunc
		jmapOptionsAndLocation = savedJmapFunc
	})

	goodGcoreOptions := gcoreOptionsAndLocation
	goodJmapOptions := jmapOptionsAndLocation
	badOptions := func(req *pb.GetMemoryDumpRequest) ([]string, string, error) {
		return nil, "", errors.New("error")
	}

	testdir, err := os.MkdirTemp("", "tests")
	testutil.FatalOnErr("can't create temp dir", err, t)
	t.Cleanup(func() { os.RemoveAll(testdir) })

	for _, test := range []struct {
		name     string
		command  string
		input    string
		options  func(req *pb.GetMemoryDumpRequest) ([]string, string, error)
		req      *pb.GetMemoryDumpRequest
		noOutput bool
		wantErr  bool
	}{
		{
			name:    "basic contents check",
			command: testutil.ResolvePath(t, "cat"),
			options: goodGcoreOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_GCORE,
			},
		},
		{
			name:    "basic contents check - java",
			command: testutil.ResolvePath(t, "cat"),
			options: goodJmapOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_JMAP,
			},
		},
		{
			name:    "basic contents check - url",
			command: testutil.ResolvePath(t, "cat"),
			options: goodGcoreOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid: 1,
				Destination: &pb.GetMemoryDumpRequest_Url{
					Url: &pb.DumpDestinationUrl{
						Url: fmt.Sprintf("file://%s", testdir),
					},
				},
				DumpType: pb.DumpType_DUMP_TYPE_GCORE,
			},
		},
		{
			name:    "No command",
			input:   "./testdata/core.test",
			options: goodGcoreOptions,
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_GCORE,
			},
			wantErr: true,
		},
		{
			name:    "No command - java",
			input:   "./testdata/core.test",
			options: goodGcoreOptions,
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_JMAP,
			},
			wantErr: true,
		},
		{
			name:    "bad pid",
			command: testutil.ResolvePath(t, "cat"),
			options: goodGcoreOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_GCORE,
			},
			wantErr: true,
		},
		{
			name:    "Bad destination - url",
			command: testutil.ResolvePath(t, "cat"),
			options: goodGcoreOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid: 1,
				Destination: &pb.GetMemoryDumpRequest_Url{
					Url: &pb.DumpDestinationUrl{},
				},
				DumpType: pb.DumpType_DUMP_TYPE_GCORE,
			},
			wantErr: true,
		},
		{
			name:    "Bad dump type - bad enum",
			command: testutil.ResolvePath(t, "cat"),
			options: goodGcoreOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_GCORE + 99,
			},
			wantErr: true,
		},
		{
			name:    "Bad options",
			command: testutil.ResolvePath(t, "cat"),
			options: badOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_GCORE,
			},
			wantErr: true,
		},
		{
			name:    "Bad options - java",
			command: testutil.ResolvePath(t, "cat"),
			options: badOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				DumpType:    pb.DumpType_DUMP_TYPE_JMAP,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
			},
			wantErr: true,
		},
		{
			name:    "Bad command",
			command: "//foo",
			options: goodGcoreOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_GCORE,
			},
			wantErr: true,
		},
		{
			name:    "Command returns error",
			command: testutil.ResolvePath(t, "false"),
			options: goodGcoreOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_GCORE,
			},
			wantErr: true,
		},
		{
			name:    "Bad output file",
			command: testutil.ResolvePath(t, "true"),
			options: goodGcoreOptions,
			input:   "./testdata/core.test",
			req: &pb.GetMemoryDumpRequest{
				Pid:         1,
				Destination: &pb.GetMemoryDumpRequest_Stream{},
				DumpType:    pb.DumpType_DUMP_TYPE_GCORE,
			},
			noOutput: true,
			wantErr:  true,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			*gcoreBin = test.command
			gcoreOptionsAndLocation = test.options
			*jmapBin = test.command
			jmapOptionsAndLocation = test.options

			// Need a tmp dir and a copy of the test input since the options
			// caller is expecting to cleanup the directory when it's done.
			dir, err := os.MkdirTemp("", "cores")
			testutil.FatalOnErr("can't make tmpdir", err, t)
			file := filepath.Join(dir, "core")
			var testdata []byte
			if !test.noOutput {
				testdata, err = os.ReadFile(test.input)
				testutil.FatalOnErr("can't read test input", err, t)
				err = os.WriteFile(file, testdata, 0666)
				testutil.FatalOnErr("can't copy test data", err, t)
			}
			testInput = file

			stream, err := client.GetMemoryDump(ctx, test.req)
			testutil.FatalOnErr("setting up stream", err, t)

			var data []byte
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Logf("%s - err: %v", test.name, err)
				}

				if got, want := err != nil, test.wantErr; got != want {
					t.Fatalf("%s: unexpected error state. got %t want %t err %v", test.name, got, want, err)
				}

				// If we got here and expected an error the response is nil so just break.
				if test.wantErr {
					break
				}
				data = append(data, resp.Data...)
			}

			// If we're not expecting an error and using a URL it didn't go to data so we need
			// to load that up for comparison.
			if !test.wantErr && test.req.GetUrl() != nil {
				// Need to query the bucket to see what we got.
				bucket, err := blob.OpenBucket(ctx, test.req.GetUrl().Url)
				testutil.FatalOnErr(fmt.Sprintf("can't open bucket %s", test.req.GetUrl().Url), err, t)
				file := fmt.Sprintf("bufconn-core.%d", test.req.Pid)
				rdr, err := bucket.NewReader(context.Background(), file, nil)
				testutil.FatalOnErr(fmt.Sprintf("can't open bucket %s key %s", test.req.GetUrl().Url, file), err, t)
				data, err = io.ReadAll(rdr)
				testutil.FatalOnErr(fmt.Sprintf("can't read all bytes from mem://%s", file), err, t)
				t.Cleanup(func() { bucket.Close() })
			}

			if !test.wantErr {
				if !bytes.Equal(testdata, data) {
					t.Fatalf("%s: Responses differ.\nGot\n%+v\n\nWant\n%+v", test.name, data, testdata)
				}
			}
		})
	}
}
