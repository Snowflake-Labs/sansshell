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

package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "gocloud.dev/blob/fileblob"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
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
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	testutil.FatalOnNoErr("empty read request (no Read or Tail)", err, t)
}

func TestRead(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
			name:      "validFile-normal",
			filename:  validFile,
			chunksize: 10,
		},
		{
			name:      "validFile-1-byte-chunk",
			filename:  validFile,
			chunksize: 1,
		},
		{
			name:      "validFile-with-offset-and-length",
			filename:  validFile,
			chunksize: 10,
			offset:    10,
			length:    15,
		},
		{
			name:      "validFile-from-end",
			filename:  validFile,
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
			filename: nonAbsolutePath,
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
				testutil.WantErr(tc.name, err, tc.wantErr, t)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	ReadTimeout = 1 * time.Second

	// Create a file with some initial data.
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("can't create tmpfile", err, t)
	data := "Some data\n"
	_, err = f1.WriteString(data)
	testutil.FatalOnErr("WriteString", err, t)
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
	time.Sleep(ReadTimeout + 1*time.Second)

	// This should cause Recv() to fail
	resp, err = stream.Recv()
	t.Log(err)
	testutil.FatalOnNoErr(fmt.Sprintf("recv with cancelled context - resp %v", resp), err, t)
}

func TestStat(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	uid, gid := uint32(os.Getuid()), uint32(os.Getgid())
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	f1Stat, err := osStat(f1.Name(), false)
	testutil.FatalOnErr("f1.Stat", err, t)

	dirStat, err := osStat(temp, false)
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
			testutil.DiffErr(tc.name, reply, tc.reply, t)
			err = stream.CloseSend()
			testutil.FatalOnErr("CloseSend", err, t)
		})
	}
}

func TestSum(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
				if err == nil || (!strings.Contains(err.Error(), "directory") && !strings.Contains(err.Error(), "copy/read error")) {
					t.Fatalf("%s : err was %v, want err containing 'directory' or 'copy/read error'", op, err)
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
			testutil.DiffErr(tc.name, reply, tc.reply, t)
			err = stream.CloseSend()
			testutil.FatalOnErr("CloseSend", err, t)
		})
	}
}

func TestSetFileAttributes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file attributes are mostly unsupported on windows")
	}

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	uid, gid := os.Getuid(), os.Getgid()
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	f1Stat, err := osStat(f1.Name(), false)
	testutil.FatalOnErr("f1.Stat", err, t)

	// Construct a directory with no perms. We'll put
	// a file in there which should make chmod fail on it.
	badDir := filepath.Join(t.TempDir(), "/foo")
	err = os.Mkdir(badDir, fs.ModePerm)
	testutil.FatalOnErr("os.Mkdir", err, t)
	f2, err := os.CreateTemp(badDir, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	err = os.Chmod(badDir, 0)
	testutil.FatalOnErr("chmod", err, t)

	setPath := ""
	setUID, setGID := 0, 0
	setImmutable, chownError, immutableError := false, false, false

	// Setup a mock chown so we can tell it's getting called
	// as expected. Plus we can force error cases then.
	savedChown := chown
	chown = func(path string, uid int, gid int) error {
		setPath = path
		setUID = uid
		setGID = gid
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
		// Needed or we panic with generated cleanup trying to remove tmp directories.
		err = os.Chmod(badDir, fs.ModePerm)
		testutil.FatalOnErr("Chmod", err, t)
	})
	// Tests below set user to nobody.
	// group has to be looked up because on some systems it's
	// named "nogroup" and on others "nobody" so just look up
	// name of nobody's primary group.
	nobodyUname := "nobody"
	nobody, err := user.Lookup(nobodyUname)
	testutil.FatalOnErr("nobody uid", err, t)
	nobodyUid, err := strconv.Atoi(nobody.Uid)
	testutil.FatalOnErr("nobody uid conv", err, t)
	nobodyGroup, err := user.LookupGroupId(nobody.Gid)
	testutil.FatalOnErr("nobody gid", err, t)
	nobodyGid, err := strconv.Atoi(nobodyGroup.Gid)
	testutil.FatalOnErr("nobody uid conv", err, t)

	for _, tc := range []struct {
		name              string
		input             *pb.SetFileAttributesRequest
		wantErr           bool
		chownError        bool
		immutableError    bool
		setUID            bool
		expectedUID       int
		setGID            bool
		expectedGID       int
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
			setUID:            true,
			expectedUID:       uid + 1,
			setGID:            true,
			expectedGID:       gid + 1,
			setMode:           true,
			expectedMode:      f1Stat.Mode + 1,
			setImmutable:      true,
			expectedImmutable: true,
		},
		{
			name: "Expected uid, gid from username, group",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f1.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Username{
								Username: nobodyUname,
							},
						},
						{
							Value: &pb.FileAttribute_Group{
								Group: nobodyGroup.Name,
							},
						},
					},
				},
			},
			setUID:      true,
			expectedUID: nobodyUid,
			setGID:      true,
			expectedGID: nobodyGid,
		},
		{
			name: "Chmod fails (path not accessible)",
			input: &pb.SetFileAttributesRequest{
				Attrs: &pb.FileAttributes{
					Filename: f2.Name(),
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Mode{
								Mode: f1Stat.Mode + 1,
							},
						},
					},
				},
			},
			wantErr: true,
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
			testutil.WantErr(tc.name, err, tc.wantErr, t)

			// Expected an error so we're done.
			if tc.wantErr {
				return
			}

			if got, want := setPath, tc.input.Attrs.Filename; got != want {
				t.Fatalf("%s: didn't use correct path. Got %s and want %s", tc.name, got, want)
			}
			if tc.setUID {
				if got, want := setUID, tc.expectedUID; got != want {
					t.Fatalf("%s: didn't use correct uid. Got %d and want %d", tc.name, got, want)
				}
			}
			if tc.setGID {
				if got, want := setGID, tc.expectedGID; got != want {
					t.Fatalf("%s: didn't use correct gid. Got %d and want %d", tc.name, got, want)
				}
			}

			if tc.setMode {
				s, err := osStat(f1.Name(), false)
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
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	f1.Close()

	symlink := filepath.Join(temp, "sym")
	testutil.FatalOnErr("osSymlink", os.Symlink("broken", symlink), t)

	dirStat, err := osStat(temp, false)
	testutil.FatalOnErr("osStat", err, t)
	f1Stat, err := osStat(f1.Name(), false)
	testutil.FatalOnErr("osStat", err, t)
	symStat, err := osStat(symlink, true)
	testutil.FatalOnErr("osStat", err, t)

	// Construct a directory with no perms. We should be able
	// to stat this but then fail to readdir on it.
	badDir := filepath.Join(t.TempDir(), "/foo")
	err = os.Mkdir(badDir, 0)
	testutil.FatalOnErr("Mkdir", err, t)

	origOsStat := osStat

	for _, tc := range []struct {
		name          string
		req           *pb.ListRequest
		osStat        func(string, bool) (*pb.StatReply, error)
		wantErr       bool
		skipOnWindows bool
		expected      []*pb.StatReply
	}{
		{
			name:    "Blank filename",
			req:     &pb.ListRequest{},
			wantErr: true,
		},
		{
			name: "Non absolute path",
			req: &pb.ListRequest{
				Entry: nonAbsolutePath,
			},
			wantErr: true,
		},
		{
			name: "valid but bad path",
			req: &pb.ListRequest{
				Entry: "/bad-file-name",
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
				symStat,
				f1Stat,
			},
		},
		{
			name: "directory open fail",
			req: &pb.ListRequest{
				Entry: badDir,
			},
			skipOnWindows: true,
			wantErr:       true,
		},
		{
			name: "stat fails inside directory",
			req: &pb.ListRequest{
				Entry: temp,
			},
			osStat: func(s string, b bool) (*pb.StatReply, error) {
				fmt.Printf("stat: %s - %s\n", s, f1.Name())
				if s == f1.Name() {
					return nil, errors.New("stat error")
				}
				return origOsStat(s, b)
			},
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipOnWindows {
				return
			}
			client := pb.NewLocalFileClient(conn)

			t.Cleanup(func() {
				osStat = origOsStat
			})

			if tc.osStat != nil {
				osStat = tc.osStat
			}
			// This end shouldn't error, when we receive from the stream is where we'll get those.
			stream, err := client.List(ctx, tc.req)
			testutil.FatalOnErr("List", err, t)
			var out []*pb.StatReply
			var gotErr error
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				t.Log(err)
				// We can't check errors here directly as we may get a few entries
				// and then an error. So record the last one we see and test outside
				// the loop.
				if err != nil {
					gotErr = err
					break
				}
				out = append(out, resp.Entry)
			}
			testutil.WantErr(tc.name, gotErr, tc.wantErr, t)
			if !tc.wantErr {
				testutil.DiffErr(tc.name, out, tc.expected, t)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	uid, gid := os.Getuid(), os.Getgid()

	var setPath string
	var setImmutable, immutableError bool
	savedChangeImmutableOS := changeImmutable
	changeImmutableOS = func(path string, immutable bool) error {
		setPath = path
		setImmutable = immutable
		if immutableError {
			return errors.New("immutable error")
		}

		return nil
	}

	t.Cleanup(func() {
		changeImmutableOS = savedChangeImmutableOS
	})

	for _, tc := range []struct {
		name              string
		reqs              []*pb.WriteRequest
		wantErr           bool
		filename          string
		validate          string
		touchFile         bool
		immutableError    bool
		validateImmutable bool
		skipOnWindows     bool
	}{
		{
			name: "Blank request",
			reqs: []*pb.WriteRequest{
				{},
			},
			wantErr: true,
		},
		{
			name: "Empty description",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{},
				},
			},
			wantErr: true,
		},
		{
			name: "Description and then blank",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Gid{
											Gid: uint32(gid),
										},
									},
									{
										Value: &pb.FileAttribute_Mode{
											Mode: 0777,
										},
									},
								},
							},
						},
					},
				},
				{},
			},
			wantErr: true,
		},
		{
			name: "Bad path",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: "/tmp/foo/../../etc/passwd",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid path",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, temp, "/file"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "contents first",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Contents{
						Contents: []byte("contents"),
					},
				},
			},
			wantErr: true,
		},
		{
			// NOTE: We don't need all the permutations of this as TestSetFileAttributes already does that.
			name: "invalid attrs - multiple uid",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "2 descriptions",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Gid{
											Gid: uint32(gid),
										},
									},
									{
										Value: &pb.FileAttribute_Mode{
											Mode: 0777,
										},
									},
								},
							},
						},
					},
				},
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Gid{
											Gid: uint32(gid),
										},
									},
									{
										Value: &pb.FileAttribute_Mode{
											Mode: 0777,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Fail due to overwrite = false",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Gid{
											Gid: uint32(gid),
										},
									},
									{
										Value: &pb.FileAttribute_Mode{
											Mode: 0777,
										},
									},
								},
							},
						},
					},
				},
				{
					Request: &pb.WriteRequest_Contents{
						Contents: []byte("contents"),
					},
				},
			},
			filename:  filepath.Join(temp, "/file"),
			wantErr:   true,
			touchFile: true,
		},
		{
			name: "Write a file",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Gid{
											Gid: uint32(gid),
										},
									},
									{
										Value: &pb.FileAttribute_Mode{
											Mode: 0777,
										},
									},
								},
							},
						},
					},
				},
				{
					Request: &pb.WriteRequest_Contents{
						Contents: []byte("contents"),
					},
				},
			},
			filename: filepath.Join(temp, "/file"),
			validate: "contents",
		},
		{
			name: "Write a file (immutable)",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Gid{
											Gid: uint32(gid),
										},
									},
									{
										Value: &pb.FileAttribute_Mode{
											Mode: 0777,
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
					},
				},
				{
					Request: &pb.WriteRequest_Contents{
						Contents: []byte("contents"),
					},
				},
			},
			filename:          filepath.Join(temp, "/file"),
			validate:          "contents",
			validateImmutable: true,
		},
		{
			name: "Overwrite succeeds",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Gid{
											Gid: uint32(gid),
										},
									},
									{
										Value: &pb.FileAttribute_Mode{
											Mode: 0777,
										},
									},
								},
							},
							Overwrite: true,
						},
					},
				},
				{
					Request: &pb.WriteRequest_Contents{
						Contents: []byte("contents"),
					},
				},
			},
			filename:  filepath.Join(temp, "/file"),
			touchFile: true,
			validate:  "contents",
		},
		{
			name: "Immutable fails",
			reqs: []*pb.WriteRequest{
				{
					Request: &pb.WriteRequest_Description{
						Description: &pb.FileWrite{
							Attrs: &pb.FileAttributes{
								Filename: filepath.Join(temp, "/file"),
								Attributes: []*pb.FileAttribute{
									{
										Value: &pb.FileAttribute_Uid{
											Uid: uint32(uid),
										},
									},
									{
										Value: &pb.FileAttribute_Gid{
											Gid: uint32(gid),
										},
									},
									{
										Value: &pb.FileAttribute_Mode{
											Mode: 0777,
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
					},
				},
				{
					Request: &pb.WriteRequest_Contents{
						Contents: []byte("contents"),
					},
				},
			},
			filename:       filepath.Join(temp, "/file"),
			wantErr:        true,
			immutableError: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if tc.filename != "" {
					os.Remove(tc.filename)
				}
			}()

			// Always reset to known values so we don't accidentally test state
			// from another run.
			setPath = ""
			setImmutable = false
			immutableError = tc.immutableError

			if tc.touchFile {
				f, err := os.Create(tc.filename)
				testutil.FatalOnErr("Create()", err, t)
				err = f.Close()
				testutil.FatalOnErr("Close()", err, t)
			}
			client := pb.NewLocalFileClient(conn)
			server, err := client.Write(context.Background())
			testutil.FatalOnErr("Write", err, t)

			for _, req := range tc.reqs {
				err := server.Send(req)
				testutil.FatalOnErr("Write send", err, t)
			}
			_, err = server.CloseAndRecv()
			t.Log(err)
			if got, want := err != nil && err != io.EOF, tc.wantErr; got != want {
				t.Fatalf("%s: error state inconsistent got %t and want %t err %v", tc.name, got, want, err)
			}
			if tc.wantErr {
				return
			}

			c, err := os.ReadFile(tc.filename)
			testutil.FatalOnErr("ReadFile()", err, t)

			if got, want := c, []byte(tc.validate); !bytes.Equal(got, want) {
				t.Fatalf("%s: contents not equal.\nGot : %s\nWant: %s", tc.name, got, want)
			}

			if tc.validateImmutable {
				if got, want := setPath, tc.filename; got != want {
					t.Fatalf("%s: immutable setting didn't do correct file. got %s and want %s", tc.name, got, want)
				}
				if got, want := setImmutable, true; got != want {
					t.Fatalf("%s: didn't set immutable state as expected. got %t and want %t", tc.name, got, want)
				}
			}
		})
	}
}

func TestCopy(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	// Setup source bucket/file and contents
	contents := "contents"
	temp := t.TempDir()
	uid, gid := os.Getuid(), os.Getgid()
	f1, err := os.CreateTemp(temp, "file")
	testutil.FatalOnErr("CreateTemp", err, t)
	if n, err := f1.WriteString(contents); n != len(contents) || err != nil {
		t.Fatalf("WriteString failed. Wrote %d, expected %d err - %v", n, len(contents), err)
	}
	err = f1.Close()
	testutil.FatalOnErr("Close", err, t)

	for _, tc := range []struct {
		name          string
		req           *pb.CopyRequest
		wantErr       bool
		filename      string
		validate      string
		touchFile     bool
		skipOnWindows bool
	}{
		{
			name:    "Blank request",
			req:     &pb.CopyRequest{},
			wantErr: true,
		},
		{
			name: "No bucket",
			req: &pb.CopyRequest{
				Destination: &pb.FileWrite{},
			},
			wantErr: true,
		},
		{
			name: "No key",
			req: &pb.CopyRequest{
				Destination: &pb.FileWrite{},
				Bucket:      fmt.Sprintf("file://%s", filepath.Dir(f1.Name())),
			},
			wantErr: true,
		},
		{
			name: "No attrs",
			req: &pb.CopyRequest{
				Destination: &pb.FileWrite{},
				Bucket:      fmt.Sprintf("file://%s", filepath.Dir(f1.Name())),
				Key:         filepath.Base(f1.Name()),
			},
			wantErr: true,
		},
		{
			// Don't need to test all attributes combinations as TestWrite did this.
			name: "Bad path",
			req: &pb.CopyRequest{
				Destination: &pb.FileWrite{
					Attrs: &pb.FileAttributes{
						Filename: "/tmp/foo/../../etc/passwd",
					},
				},
				Bucket: fmt.Sprintf("file://%s", filepath.Dir(f1.Name())),
				Key:    filepath.Base(f1.Name()),
			},
			wantErr: true,
		},
		{
			name: "Bad bucket",
			req: &pb.CopyRequest{
				Destination: &pb.FileWrite{
					Attrs: &pb.FileAttributes{
						Filename: filepath.Join(temp, "/file"),
					},
				},
				Bucket: "fil://not-a-path",
				Key:    filepath.Base(f1.Name()),
			},
			wantErr: true,
		},
		{
			name: "Bad key",
			req: &pb.CopyRequest{
				Destination: &pb.FileWrite{
					Attrs: &pb.FileAttributes{
						Filename: filepath.Join(temp, "/file"),
					},
				},
				Bucket: "file://not-a-path",
				Key:    filepath.Base(f1.Name()),
			},
			wantErr: true,
		},
		{
			// Same here. We don't need to test all finalization permutations as TestWrite did that.
			name: "Fail due to overwrite = false",
			req: &pb.CopyRequest{
				Destination: &pb.FileWrite{
					Attrs: &pb.FileAttributes{
						Filename: filepath.Join(temp, "/file"),
						Attributes: []*pb.FileAttribute{
							{
								Value: &pb.FileAttribute_Uid{
									Uid: uint32(uid),
								},
							},
							{
								Value: &pb.FileAttribute_Gid{
									Gid: uint32(gid),
								},
							},
							{
								Value: &pb.FileAttribute_Mode{
									Mode: 0777,
								},
							},
						},
					},
				},
				Bucket: fmt.Sprintf("file://%s", filepath.Dir(f1.Name())),
				Key:    filepath.Base(f1.Name()),
			},
			filename:  filepath.Join(temp, "/file"),
			wantErr:   true,
			touchFile: true,
		},

		{
			name: "Copy a file",
			req: &pb.CopyRequest{
				Destination: &pb.FileWrite{
					Attrs: &pb.FileAttributes{
						Filename: filepath.Join(temp, "/file"),
						Attributes: []*pb.FileAttribute{
							{
								Value: &pb.FileAttribute_Uid{
									Uid: uint32(uid),
								},
							},
							{
								Value: &pb.FileAttribute_Gid{
									Gid: uint32(gid),
								},
							},
							{
								Value: &pb.FileAttribute_Mode{
									Mode: 0777,
								},
							},
						},
					},
				},
				Bucket: fmt.Sprintf("file://%s", filepath.Dir(f1.Name())),
				Key:    filepath.Base(f1.Name()),
			},
			validate:      contents,
			filename:      filepath.Join(temp, "file"),
			skipOnWindows: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipOnWindows && runtime.GOOS == "windows" {
				return
			}

			defer func() {
				if tc.filename != "" {
					os.Remove(tc.filename)
				}
			}()

			if tc.touchFile {
				f, err := os.Create(tc.filename)
				testutil.FatalOnErr("Create()", err, t)
				err = f.Close()
				testutil.FatalOnErr("Close()", err, t)
			}

			client := pb.NewLocalFileClient(conn)
			_, err := client.Copy(context.Background(), tc.req)
			t.Log(err)
			testutil.WantErr(tc.name, err, tc.wantErr, t)
			if tc.wantErr {
				return
			}

			c, err := os.ReadFile(tc.filename)
			testutil.FatalOnErr("ReadFile()", err, t)

			if got, want := c, []byte(tc.validate); !bytes.Equal(got, want) {
				t.Fatalf("%s: contents not equal.\nGot : %s\nWant: %s", tc.name, got, want)
			}
		})
	}
}

func TestRm(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	badDir := filepath.Join(temp, "bad")
	err = os.Mkdir(badDir, fs.ModePerm)
	testutil.FatalOnErr("os.Mkdir", err, t)
	f2, err := os.CreateTemp(badDir, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	err = os.Chmod(badDir, 0)
	testutil.FatalOnErr("Chmod", err, t)
	f1.Close()
	f2.Close()

	t.Cleanup(func() {
		// Needed or we panic with generated cleanup trying to remove tmp directories.
		err = os.Chmod(badDir, fs.ModePerm)
		testutil.FatalOnErr("Chmod", err, t)
	})

	for _, tc := range []struct {
		name          string
		filename      string
		skipOnWindows bool
		wantErr       bool
	}{
		{
			name:     "bad path",
			filename: nonAbsolutePath,
			wantErr:  true,
		},
		{
			name:          "bad permissions to file",
			filename:      f2.Name(),
			skipOnWindows: true,
			wantErr:       true,
		},
		{
			name:     "working remove",
			filename: f1.Name(),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipOnWindows {
				return
			}
			client := pb.NewLocalFileClient(conn)
			_, err := client.Rm(ctx, &pb.RmRequest{Filename: tc.filename})
			testutil.WantErr(tc.name, err, tc.wantErr, t)
		})
	}
}

func TestRmdir(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	dir := filepath.Join(temp, "/dir")
	err = os.Mkdir(dir, fs.ModePerm)
	testutil.FatalOnErr("os.Mkdir", err, t)
	badDir := filepath.Join(temp, "/bad")
	err = os.Mkdir(badDir, fs.ModePerm)
	testutil.FatalOnErr("os.Mkdir", err, t)
	failDir := filepath.Join(temp, "/bad", "/bad")
	err = os.Mkdir(failDir, 0)
	testutil.FatalOnErr("os.Mkdir", err, t)
	err = os.Chmod(badDir, 0)
	testutil.FatalOnErr("Chmod", err, t)

	t.Cleanup(func() {
		// Needed or we panic with generated cleanup trying to remove tmp directories.
		err = os.Chmod(badDir, fs.ModePerm)
		testutil.FatalOnErr("Chmod", err, t)
	})

	for _, tc := range []struct {
		name          string
		directory     string
		skipOnWindows bool
		wantErr       bool
	}{
		{
			name:      "bad path",
			directory: "/tmp/foo/../../etc",
			wantErr:   true,
		},
		{
			name:          "bad permissions to directory",
			directory:     failDir,
			skipOnWindows: true,
			wantErr:       true,
		},
		{
			name:      "working remove",
			directory: dir,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipOnWindows && runtime.GOOS == "windows" {
				return
			}
			client := pb.NewLocalFileClient(conn)
			_, err := client.Rmdir(ctx, &pb.RmdirRequest{Directory: tc.directory})
			testutil.WantErr(tc.name, err, tc.wantErr, t)
		})
	}
}

func TestRename(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	dir := filepath.Join(temp, "/dir")
	err = os.Mkdir(dir, fs.ModePerm)
	testutil.FatalOnErr("os.Mkdir", err, t)
	badDir := filepath.Join(temp, "/bad")
	err = os.Mkdir(badDir, 0)
	testutil.FatalOnErr("os.Mkdir", err, t)

	t.Cleanup(func() {
		// Needed or we panic with generated cleanup trying to remove tmp directories.
		err = os.Chmod(badDir, fs.ModePerm)
		testutil.FatalOnErr("Chmod", err, t)
	})

	for _, tc := range []struct {
		name    string
		req     *pb.RenameRequest
		wantErr bool
	}{
		{
			name: "bad original path",
			req: &pb.RenameRequest{
				OriginalName: "/tmp/foo/../../etc/passwd",
			},
			wantErr: true,
		},
		{
			name: "bad destination path",
			req: &pb.RenameRequest{
				OriginalName:    "/tmp/foo",
				DestinationName: "/tmp/bar/../../etc/passwd",
			},
			wantErr: true,
		},
		{
			name: "bad permissions to directory",
			req: &pb.RenameRequest{
				OriginalName:    filepath.Join(temp, "file"),
				DestinationName: badDir,
			},
			wantErr: true,
		},
		{
			name: "working rename",
			req: &pb.RenameRequest{
				OriginalName:    filepath.Join(temp, "file"),
				DestinationName: filepath.Join(temp, "newfile"),
			},
		},
		{
			name: "rename a directory",
			req: &pb.RenameRequest{
				OriginalName:    dir,
				DestinationName: filepath.Join(temp, "newdir"),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := os.WriteFile(filepath.Join(temp, "file"), []byte("contents"), 0644)
			testutil.FatalOnErr("WriteFile", err, t)
			defer os.Remove(filepath.Join(temp, "file"))
			client := pb.NewLocalFileClient(conn)
			_, err = client.Rename(ctx, tc.req)
			testutil.WantErr(tc.name, err, tc.wantErr, t)
			if tc.wantErr {
				return
			}
			_, err = os.Stat(tc.req.OriginalName)
			testutil.FatalOnNoErr("original remains", err, t)
			_, err = os.Stat(tc.req.DestinationName)
			testutil.FatalOnErr("destination doesn't exist", err, t)
			os.Remove(tc.req.DestinationName)
		})
	}
}

func TestReadlink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("os.CreateTemp", err, t)
	t.Cleanup(func() { f1.Close() })

	symlink := filepath.Join(temp, "sym")
	testutil.FatalOnErr("osSymlink", os.Symlink(f1.Name(), symlink), t)
	broken := filepath.Join(temp, "broken")
	testutil.FatalOnErr("osSymlink", os.Symlink("nonexistent", broken), t)

	for _, tc := range []struct {
		name     string
		req      *pb.ReadlinkRequest
		wantErr  bool
		expected *pb.ReadlinkReply
	}{
		{
			name:    "Blank filename",
			req:     &pb.ReadlinkRequest{},
			wantErr: true,
		},
		{
			name: "Non absolute path",
			req: &pb.ReadlinkRequest{
				Filename: nonAbsolutePath,
			},
			wantErr: true,
		},
		{
			name: "Normal file",
			req: &pb.ReadlinkRequest{
				Filename: f1.Name(),
			},
			wantErr: true,
		},
		{
			name: "Normal symlink",
			req: &pb.ReadlinkRequest{
				Filename: symlink,
			},
			expected: &pb.ReadlinkReply{
				Linkvalue: f1.Name(),
			},
		},
		{
			name: "Broken symlink",
			req: &pb.ReadlinkRequest{
				Filename: broken,
			},
			expected: &pb.ReadlinkReply{
				Linkvalue: "nonexistent",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := pb.NewLocalFileClient(conn)

			resp, err := client.Readlink(ctx, tc.req)
			testutil.WantErr(tc.name, err, tc.wantErr, t)
			if !tc.wantErr {
				testutil.DiffErr(tc.name, resp, tc.expected, t)
			}
		})
	}
}

func TestSymlink(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	for _, tc := range []struct {
		name    string
		req     *pb.SymlinkRequest
		wantErr bool
	}{
		{
			name:    "Blank filename",
			req:     &pb.SymlinkRequest{},
			wantErr: true,
		},
		{
			name: "Non absolute linkname",
			req: &pb.SymlinkRequest{
				Target:   "foo",
				Linkname: "/tmp/../symlink",
			},
			wantErr: true,
		},
		{
			name: "Valid request",
			req: &pb.SymlinkRequest{
				Target:   "foo",
				Linkname: filepath.Join(t.TempDir(), "symlink"),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := pb.NewLocalFileClient(conn)

			_, err := client.Symlink(ctx, tc.req)
			testutil.WantErr(tc.name, err, tc.wantErr, t)
		})
	}
}
