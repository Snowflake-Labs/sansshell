package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
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

func TestEmptyRead(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewLocalFileClient(conn)
	// We won't get an error here as we have to read from the stream.
	stream, err := client.Read(ctx, &pb.ReadActionRequest{})
	testutil.FatalOnErr("Read", err, t)
	_, err = stream.Recv()
	if err == io.EOF {
		t.Fatal("Expected a real error, not EOF")
	}
	if err == nil {
		t.Fatal("Expected an error from an empty read request (no Read or Tail) and not nothing")
	}

}

func TestRead(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	for _, tc := range []struct {
		name      string
		filename  string
		wantErr   bool
		chunksize int
		offset    int64
		length    int64
	}{
		{
			name:      "/etc/hosts-normal",
			filename:  "/etc/hosts",
			chunksize: 10,
		},
		{
			name:      "/etc/hosts-1-byte-chunk",
			filename:  "/etc/hosts",
			chunksize: 1,
		},
		{
			name:      "/etc/hosts-with-offset-and-length",
			filename:  "/etc/hosts",
			chunksize: 10,
			offset:    10,
			length:    15,
		},
		{
			name:      "/etc/hosts-from-end",
			filename:  "/etc/hosts",
			chunksize: 10,
			offset:    -20,
			length:    15,
		},
		{
			name:     "bad-file",
			filename: "/no-such-filename-for-sansshell-unittest",
			wantErr:  true,
		},
		{
			name:     "non-absolute file",
			filename: "/tmp/foo/../../etc/passwd",
			wantErr:  true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			oldChunk := util.StreamingChunkSize
			t.Cleanup(func() {
				util.StreamingChunkSize = oldChunk
			})

			util.StreamingChunkSize = tc.chunksize

			client := pb.NewLocalFileClient(conn)

			stream, err := client.Read(ctx, &pb.ReadActionRequest{
				Request: &pb.ReadActionRequest_File{
					File: &pb.ReadRequest{
						Filename: tc.filename,
						Offset:   tc.offset,
						Length:   tc.length,
					},
				},
			})
			// In general this can only fail here for connection issues which
			// we're not expecting. Actual fails happen in Recv below.
			testutil.FatalOnErr("Read failed", err, t)

			buf := &bytes.Buffer{}
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if got, want := err != nil, tc.wantErr; got != want {
					t.Fatalf("%s: error state inconsistent got %t and want %t err %v", tc.name, got, want, err)
				}
				if tc.wantErr {
					// If this was an expected error we're done.
					return
				}

				t.Logf("Response: %+v", resp)
				contents := resp.Contents
				n, err := buf.Write(contents)
				if got, want := n, len(contents); got != want {
					t.Fatalf("Can't write into buffer at correct length. Got %d want %d", got, want)
				}
				testutil.FatalOnErr("can't write into buffer", err, t)
			}
			contents, err := os.ReadFile(tc.filename)
			testutil.FatalOnErr("reading test data", err, t)
			if tc.offset != 0 || tc.length != 0 {
				start := 0
				if tc.offset > 0 {
					start = int(tc.offset)
				}
				if tc.offset < 0 {
					start = len(contents) + int(tc.offset)
				}
				length := 0
				if tc.length != 0 {
					length = int(tc.length)
				}
				contents = contents[start : start+length]
			}
			if got, want := buf.Bytes(), contents; !bytes.Equal(got, want) {
				t.Fatalf("contents do not match. Got:\n%s\n\nWant:\n%s", got, want)
			}
		})
	}
}

func TestTail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	READ_TIMEOUT = 1 * time.Second

	// Create a file with some initial data.
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("can't create tmpfile", err, t)
	data := "Some data\n"
	f1.WriteString(data)
	name := f1.Name()
	err = f1.Close()
	testutil.FatalOnErr("closing file", err, t)
	client := pb.NewLocalFileClient(conn)
	stream, err := client.Read(ctx, &pb.ReadActionRequest{
		Request: &pb.ReadActionRequest_Tail{
			Tail: &pb.TailRequest{
				Filename: name,
			},
		},
	})
	testutil.FatalOnErr("error from read", err, t)

	buf := &bytes.Buffer{}
	resp, err := stream.Recv()
	testutil.FatalOnErr("error reading from stream", err, t)

	// We don't care about errors as we compare the buf later.
	buf.Write(resp.Contents)

	f1, err = os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0644)
	testutil.FatalOnErr("can't open for adding data", err, t)

	n, err := f1.WriteString(data)
	if n != len(data) || err != nil {
		t.Fatalf("incorrect length or error. Wrote %d expected %d. Error - %v", n, len(data), err)
	}
	err = f1.Close()
	testutil.FatalOnErr("close", err, t)

	resp, err = stream.Recv()
	testutil.FatalOnErr("error reading from stream", err, t)
	buf.Write(resp.Contents)
	data = data + data
	if got, want := buf.String(), data; got != want {
		t.Fatalf("didn't get matching data for second file read. Got %q and want %q", got, want)
	}

	// Now cancel our context.
	cancel()

	// Pause n+1s to make sure the server goes through a poll loop if we're
	// not on an OS that can immediately return.
	time.Sleep(READ_TIMEOUT + 1*time.Second)

	// This should cause Recv() to fail
	resp, err = stream.Recv()
	t.Log(err)
	if err == nil {
		t.Fatalf("Didn't get error from recv. Got resp: %+v", resp)
	}
}

func TestStat(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	uid, gid := uint32(os.Getuid()), uint32(os.Getgid())
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	f1Stat, err := osStat(f1.Name())
	testutil.FatalOnErr("f1.Stat", err, t)

	dirStat, err := osStat(temp)
	testutil.FatalOnErr("os.Stat(tempdir)", err, t)

	for _, tc := range []struct {
		name        string
		req         *pb.StatRequest
		reply       *pb.StatReply
		sendErrFunc func(string, error, *testing.T)
		recvErrFunc func(string, error, *testing.T)
	}{
		{
			name:        "nonexistent file",
			req:         &pb.StatRequest{Filename: "/no/such/file"},
			reply:       nil,
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: func(op string, err error, t *testing.T) {
				if err == nil {
					t.Fatalf("%s: err was nil, expecting not found", op)
				}
			},
		},
		{
			name:        "non-absolute path",
			req:         &pb.StatRequest{Filename: "../../relative-path/not-valid"},
			reply:       nil,
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: func(op string, err error, t *testing.T) {
				if !errors.Is(err, AbsolutePathError) {
					t.Fatalf("%s: err was %v, want AbsolutePathError", op, err)
				}
			},
		},
		{
			name: "directory",
			req:  &pb.StatRequest{Filename: temp},
			reply: &pb.StatReply{
				Filename:  temp,
				Size:      dirStat.Size,
				Mode:      dirStat.Mode,
				Modtime:   dirStat.Modtime,
				Uid:       uid,
				Gid:       gid,
				Immutable: dirStat.Immutable,
			},
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: testutil.FatalOnErr,
		},
		{
			name: "known test file",
			req:  &pb.StatRequest{Filename: f1.Name()},
			reply: &pb.StatReply{
				Filename:  f1.Name(),
				Size:      f1Stat.Size,
				Mode:      f1Stat.Mode,
				Modtime:   f1Stat.Modtime,
				Uid:       uid,
				Gid:       gid,
				Immutable: f1Stat.Immutable,
			},
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: testutil.FatalOnErr,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := pb.NewLocalFileClient(conn)
			stream, err := client.Stat(ctx)
			testutil.FatalOnErr("client.Stat", err, t)
			err = stream.Send(tc.req)
			if tc.sendErrFunc != nil {
				tc.sendErrFunc("stream.Send", err, t)
			}
			reply, err := stream.Recv()
			if tc.recvErrFunc != nil {
				tc.recvErrFunc("stream.Recv", err, t)
			}
			if diff := cmp.Diff(tc.reply, reply, protocmp.Transform()); diff != "" {
				t.Fatalf("%s mismatch: (-want, +got)\n%s", tc.name, diff)
			}
			err = stream.CloseSend()
			testutil.FatalOnErr("CloseSend", err, t)
		})
	}
}

func TestSum(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	tempfile, err := os.CreateTemp(temp, "testfile.*")
	t.Cleanup(func() { tempfile.Close() })
	testutil.FatalOnErr("os.CreateTemp", err, t)

	in, err := os.Open("./testdata/sum.data")
	testutil.FatalOnErr("os.Open", err, t)
	t.Cleanup(func() { in.Close() })

	_, err = io.Copy(tempfile, in)
	testutil.FatalOnErr("io.Copy", err, t)

	for _, tc := range []struct {
		name        string
		req         *pb.SumRequest
		reply       *pb.SumReply
		sendErrFunc func(string, error, *testing.T)
		recvErrFunc func(string, error, *testing.T)
	}{
		{
			name:        "nonexistent file",
			req:         &pb.SumRequest{Filename: "/no/such/file"},
			reply:       nil,
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: func(op string, err error, t *testing.T) {
				if err == nil {
					t.Fatalf("%s: err was nil, expecting not found", op)
				}
			},
		},
		{
			name:        "non-absolute path",
			req:         &pb.SumRequest{Filename: "../../relative-path/not-valid"},
			reply:       nil,
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: func(op string, err error, t *testing.T) {
				if !errors.Is(err, AbsolutePathError) {
					t.Fatalf("%s: err was %v, want AbsolutePathError", op, err)
				}
			},
		},
		{
			name:        "directory",
			req:         &pb.SumRequest{Filename: temp, SumType: pb.SumType_SUM_TYPE_SHA256},
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: func(op string, err error, t *testing.T) {
				if err == nil || !strings.Contains(err.Error(), "directory") {
					t.Fatalf("%s : err was %v, want err containing 'directory'", op, err)
				}
			},
		},
		{
			name: "default type if unset",
			req:  &pb.SumRequest{Filename: tempfile.Name()},
			reply: &pb.SumReply{
				Filename: tempfile.Name(),
				SumType:  pb.SumType_SUM_TYPE_SHA256,
				Sum:      "3115e68dae98b7c1093fbcb4173483c4af25fd7167169be1b50d9798f4e9229f",
			},
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: testutil.FatalOnErr,
		},
		{
			name:        "invalid type",
			req:         &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_SHA256 + 999},
			reply:       nil,
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: func(op string, err error, t *testing.T) {
				if err == nil {
					t.Fatalf("%s: err was nil, expected one for invalid SumType", op)
				}
			},
		},
		{
			name: "sha256",
			req:  &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_SHA256},
			reply: &pb.SumReply{
				Filename: tempfile.Name(),
				SumType:  pb.SumType_SUM_TYPE_SHA256,
				Sum:      "3115e68dae98b7c1093fbcb4173483c4af25fd7167169be1b50d9798f4e9229f",
			},
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: testutil.FatalOnErr,
		},
		{
			name: "sha512_256",
			req:  &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_SHA512_256},
			reply: &pb.SumReply{
				Filename: tempfile.Name(),
				SumType:  pb.SumType_SUM_TYPE_SHA512_256,
				Sum:      "f28247cd3fb739c77014b33f3aff1e48e7dc3674c46c10498dc8d25f4b3405a1",
			},
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: testutil.FatalOnErr,
		},
		{
			name: "md5",
			req:  &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_MD5},
			reply: &pb.SumReply{
				Filename: tempfile.Name(),
				SumType:  pb.SumType_SUM_TYPE_MD5,
				Sum:      "485032cb71937bed2d371731498d20d3",
			},
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: testutil.FatalOnErr,
		},
		{
			name: "crc32_ieee",
			req:  &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_CRC32IEEE},
			reply: &pb.SumReply{
				Filename: tempfile.Name(),
				SumType:  pb.SumType_SUM_TYPE_CRC32IEEE,
				Sum:      "01df4a25",
			},
			sendErrFunc: testutil.FatalOnErr,
			recvErrFunc: testutil.FatalOnErr,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := pb.NewLocalFileClient(conn)
			stream, err := client.Sum(ctx)
			testutil.FatalOnErr("client.Sum", err, t)
			err = stream.Send(tc.req)
			if tc.sendErrFunc != nil {
				tc.sendErrFunc("stream.Send", err, t)
			}
			reply, err := stream.Recv()
			if tc.recvErrFunc != nil {
				tc.recvErrFunc("stream.Recv", err, t)
			}
			if diff := cmp.Diff(tc.reply, reply, protocmp.Transform()); diff != "" {
				t.Fatalf("%s mismatch: (-want, +got)\n%s", tc.name, diff)
			}
			err = stream.CloseSend()
			testutil.FatalOnErr("CloseSend", err, t)
		})
	}
}

func TestSetFileAttributes(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	uid, gid := os.Getuid(), os.Getgid()
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	f1Stat, err := osStat(f1.Name())
	testutil.FatalOnErr("f1.Stat", err, t)

	setPath := ""
	setUid, setGid := 0, 0
	setImmutable, chownError, immutableError := false, false, false

	// Setup a mock chown so we can tell it's getting called
	// as expected. Plus we can force error cases then.
	savedChown := chown
	chown = func(path string, uid int, gid int) error {
		setPath = path
		setUid = uid
		setGid = gid
		if chownError {
			return errors.New("chown error")
		}
		return nil
	}
	savedChangeImmutableOS := changeImmutable
	callImmutableOS := false
	changeImmutableOS = func(path string, immutable bool) error {
		setPath = path
		setImmutable = immutable
		if immutableError {
			return errors.New("immutable error")
		}
		if callImmutableOS {
			return savedChangeImmutableOS(path, immutable)
		}
		return nil
	}

	t.Cleanup(func() {
		chown = savedChown
		changeImmutableOS = savedChangeImmutableOS
	})

	for _, tc := range []struct {
		name              string
		input             *pb.SetFileAttributesRequest
		wantErr           bool
		chownError        bool
		immutableError    bool
		setUid            bool
		expectedUid       int
		setGid            bool
		expectedGid       int
		setMode           bool
		expectedMode      uint32
		setImmutable      bool
		expectedImmutable bool
		callImmutableOS   bool
	}{
		{
			name:    "empty request",
			input:   &pb.SetFileAttributesRequest{},
			wantErr: true,
		},
		{
			name: "no path",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{},
			},
			wantErr: true,
		},
		{
			name: "non absolute path",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: "etc/passwd",
				},
			},
			wantErr: true,
		},
		{
			name: "non clean path",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: "/tmp/foo/../../etc/passwd",
				},
			},
			wantErr: true,
		},
		{
			name: "multiple uid",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Uid{
								Uid: 1,
							},
						},
						{
							Value: &pb.FileAttribute_Uid{
								Uid: 2,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple gid",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Gid{
								Gid: 1,
							},
						},
						{
							Value: &pb.FileAttribute_Gid{
								Gid: 2,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple mode",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Mode{
								Mode: 1,
							},
						},
						{
							Value: &pb.FileAttribute_Mode{
								Mode: 2,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple immutable",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Immutable{
								Immutable: true,
							},
						},
						{
							Value: &pb.FileAttribute_Immutable{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "chown error",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Uid{
								Uid: uint32(uid + 1),
							},
						},
					},
				},
			},
			chownError: true,
			wantErr:    true,
		},
		{
			name: "immutable error",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Immutable{},
						},
					},
				},
			},
			immutableError: true,
			wantErr:        true,
		},
		{
			name: "Expected path, uid, gid, mode, immutable",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Uid{
								Uid: uint32(uid + 1),
							},
						},
						{
							Value: &pb.FileAttribute_Gid{
								Gid: uint32(gid + 1),
							},
						},
						{
							Value: &pb.FileAttribute_Mode{
								Mode: f1Stat.Mode + 1,
							},
						},
						{
							Value: &pb.FileAttribute_Immutable{
								Immutable: true,
							},
						},
					},
				},
			},
			setUid:            true,
			expectedUid:       uid + 1,
			setGid:            true,
			expectedGid:       gid + 1,
			setMode:           true,
			expectedMode:      f1Stat.Mode + 1,
			setImmutable:      true,
			expectedImmutable: true,
		},
		{
			name: "Call OS immutable function",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Immutable{
								Immutable: true,
							},
						},
					},
				},
			},
			wantErr:         true,
			callImmutableOS: true,
		},
		{
			name: "Mode clips to 12 bits",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Mode{
								Mode: 01000777,
							},
						},
					},
				},
			},
			setMode:      true,
			expectedMode: f1Stat.Mode | 0777,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := pb.NewLocalFileClient(conn)
			chownError = tc.chownError
			immutableError = tc.immutableError
			callImmutableOS = tc.callImmutableOS

			_, err := client.SetFileAttributes(ctx, tc.input)
			t.Log(err)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Fatalf("%s: error state inconsistent got %t and want %t err %v", tc.name, got, want, err)
			}

			// Expected an error so we're done.
			if tc.wantErr {
				return
			}

			if got, want := setPath, tc.input.Attrs.Filename; got != want {
				t.Fatalf("%s: didn't use correct path. Got %s and want %s", tc.name, got, want)
			}
			if tc.setUid {
				if got, want := setUid, tc.expectedUid; got != want {
					t.Fatalf("%s: didn't use correct uid. Got %d and want %d", tc.name, got, want)
				}
			}
			if tc.setGid {
				if got, want := setGid, tc.expectedGid; got != want {
					t.Fatalf("%s: didn't use correct gid. Got %d and want %d", tc.name, got, want)
				}
			}

			if tc.setMode {
				s, err := osStat(f1.Name())
				testutil.FatalOnErr("stat tmp file", err, t)
				if got, want := s.Mode, tc.expectedMode; got != want {
					t.Fatalf("%s: didn't use correct mode. Got %O and want %O", tc.name, got, want)
				}
			}

			if tc.setImmutable {
				if got, want := setImmutable, tc.expectedImmutable; got != want {
					t.Fatalf("%s: didn't use correct immutable setting. Got %t and want %t", tc.name, got, want)
				}
			}
		})
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)

	dirStat, err := osStat(temp)
	testutil.FatalOnErr("osStat", err, t)
	f1Stat, err := osStat(f1.Name())
	testutil.FatalOnErr("osStat", err, t)

	for _, tc := range []struct {
		name     string
		req      *pb.ListRequest
		wantErr  bool
		expected []*pb.StatReply
	}{
		{
			name:    "Blank filename",
			req:     &pb.ListRequest{},
			wantErr: true,
		},
		{
			name: "Non absolute path",
			req: &pb.ListRequest{
				Entry: "/tmp/foo/../../etc/passwd",
			},
			wantErr: true,
		},
		{
			name: "file ls",
			req: &pb.ListRequest{
				Entry: f1.Name(),
			},
			expected: []*pb.StatReply{
				f1Stat,
			},
		},
		{
			name: "directory ls",
			req: &pb.ListRequest{
				Entry: temp,
			},
			expected: []*pb.StatReply{
				dirStat,
				f1Stat,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := pb.NewLocalFileClient(conn)

			// This end shouldn't error, when we receive from the stream is where we'll get those.
			stream, err := client.List(ctx, tc.req)
			testutil.FatalOnErr("List", err, t)

			if tc.wantErr {
				return
			}

			var out []*pb.StatReply
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				t.Log(err)
				if got, want := err != nil, tc.wantErr; got != want {
					t.Fatalf("%s: error state inconsistent got %t and want %t err %v", tc.name, got, want, err)
				}
				if tc.wantErr {
					return
				}
				out = append(out, resp.Entry)
			}
			if diff := cmp.Diff(tc.expected, out, protocmp.Transform()); diff != "" {
				t.Logf("%+v", out)
				t.Fatalf("%s mismatch: (-want, +got)\n%s", tc.name, diff)
			}
		})
	}
}
