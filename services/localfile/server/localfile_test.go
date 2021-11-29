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

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func TestRead(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	for _, tc := range []struct {
		Name      string
		Filename  string
		WantErr   bool
		Chunksize int
		Offset    int64
		Length    int64
	}{
		{
			Name:      "/etc/hosts-normal",
			Filename:  "/etc/hosts",
			Chunksize: 10,
		},
		{
			Name:      "/etc/hosts-1-byte-chunk",
			Filename:  "/etc/hosts",
			Chunksize: 1,
		},
		{
			Name:      "/etc/hosts-with-offset-and-length",
			Filename:  "/etc/hosts",
			Chunksize: 10,
			Offset:    10,
			Length:    15,
		},
		{
			Name:      "/etc/hosts-from-end",
			Filename:  "/etc/hosts",
			Chunksize: 10,
			Offset:    -20,
			Length:    15,
		},
		{
			Name:     "bad-file",
			Filename: "/no-such-filename-for-sansshell-unittest",
			WantErr:  true,
		},
		{
			Name:     "non-absolute file",
			Filename: "/tmp/foo/../../etc/passwd",
			WantErr:  true,
		},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			oldChunk := util.StreamingChunkSize
			defer func() {
				util.StreamingChunkSize = oldChunk
			}()

			util.StreamingChunkSize = tc.Chunksize

			client := pb.NewLocalFileClient(conn)

			stream, err := client.Read(ctx, &pb.ReadActionRequest{
				Request: &pb.ReadActionRequest_File{
					File: &pb.ReadRequest{
						Filename: tc.Filename,
						Offset:   tc.Offset,
						Length:   tc.Length,
					},
				},
			})
			// In general this can only fail here for connection issues which
			// we're not expecting. Actual failes happen in Recv below.
			if err != nil {
				t.Fatalf("%s: Read failed: %v", tc.Name, err)
			}

			buf := &bytes.Buffer{}
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if got, want := err != nil, tc.WantErr; got != want {
					t.Errorf("%s: error state inconsistent got %t and want %t err %v", tc.Name, got, want, err)
				}
				if tc.WantErr {
					// If this was an expected error we're done.
					return
				}

				t.Logf("Response: %+v", resp)
				contents := resp.Contents
				n, err := buf.Write(contents)
				if got, want := n, len(contents); got != want {
					t.Fatalf("Can't write into buffer at correct length. Got %d want %d", got, want)
				}
				if err != nil {
					t.Fatalf("Can't write into buffer: %v", err)
				}
			}
			contents, err := os.ReadFile(tc.Filename)
			if err != nil {
				t.Fatalf("reading test data: %s", err)
			}
			if tc.Offset != 0 || tc.Length != 0 {
				start := 0
				if tc.Offset > 0 {
					start = int(tc.Offset)
				}
				if tc.Offset < 0 {
					start = len(contents) + int(tc.Offset)
				}
				length := 0
				if tc.Length != 0 {
					length = int(tc.Length)
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
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	// Create a file with some initial data.
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	if err != nil {
		t.Fatalf("Can't create tmpfile: %v", err)
	}
	data := "Some data\n"
	f1.WriteString(data)
	name := f1.Name()
	if err := f1.Close(); err != nil {
		t.Fatalf("Error closeing %s - %v", name, err)
	}
	client := pb.NewLocalFileClient(conn)
	stream, err := client.Read(ctx, &pb.ReadActionRequest{
		Request: &pb.ReadActionRequest_Tail{
			Tail: &pb.TailRequest{
				Filename: name,
			},
		},
	})
	if err != nil {
		t.Fatalf("Error from Read: %v", err)
	}

	buf := &bytes.Buffer{}
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("error reading from stream: %v", err)
	}
	// We don't care about errors as we compare the buf later.
	buf.Write(resp.Contents)

	f1, err = os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("can't open %s for adding data: %v", name, err)
	}

	n, err := f1.WriteString(data)
	if n != len(data) || err != nil {
		t.Fatalf("incorrect length or error. Wrote %d expected %d. Error - %v", n, len(data), err)
	}
	if err = f1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	resp, err = stream.Recv()
	if err != nil {
		t.Fatalf("error reading from stream: %v", err)
	}
	buf.Write(resp.Contents)
	data = data + data
	if got, want := buf.String(), data; got != want {
		t.Fatalf("didn't get matching data for second file read. Got %q and want %q", got, want)
	}
}

func fatalOnErr(op string, e error, t *testing.T) {
	t.Helper()
	if e != nil {
		t.Fatalf("%s: err was %v, want nil", op, e)
	}
}

func TestStat(t *testing.T) {
	uid, gid := uint32(os.Getuid()), uint32(os.Getgid())
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	fatalOnErr("os.CreateTemp", err, t)
	f1Stat, err := f1.Stat()
	fatalOnErr("f1.Stat", err, t)

	dirStat, err := os.Stat(temp)
	fatalOnErr("os.Stat(tempdir)", err, t)

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	fatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

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
			sendErrFunc: fatalOnErr,
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
			sendErrFunc: fatalOnErr,
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
				Filename: temp,
				Size:     dirStat.Size(),
				Mode:     uint32(dirStat.Mode()),
				Modtime:  timestamppb.New(dirStat.ModTime()),
				Uid:      uid,
				Gid:      gid,
			},
			sendErrFunc: fatalOnErr,
			recvErrFunc: fatalOnErr,
		},
		{
			name: "known test file",
			req:  &pb.StatRequest{Filename: f1.Name()},
			reply: &pb.StatReply{
				Filename: f1.Name(),
				Size:     f1Stat.Size(),
				Mode:     uint32(f1Stat.Mode()),
				Modtime:  timestamppb.New(f1Stat.ModTime()),
				Uid:      uid,
				Gid:      gid,
			},
			sendErrFunc: fatalOnErr,
			recvErrFunc: fatalOnErr,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := pb.NewLocalFileClient(conn)
			stream, err := client.Stat(ctx)
			fatalOnErr("client.Stat", err, t)
			err = stream.Send(tc.req)
			if tc.sendErrFunc != nil {
				tc.sendErrFunc("stream.Send", err, t)
			}
			reply, err := stream.Recv()
			if tc.recvErrFunc != nil {
				tc.recvErrFunc("stream.Recv", err, t)
			}
			// We can't really test this in a unit test since it usually
			// requires being root to clear so just pass through.
			if reply != nil {
				tc.reply.Immutable = reply.Immutable
			}
			if diff := cmp.Diff(tc.reply, reply, protocmp.Transform()); diff != "" {
				t.Fatalf("%s mismatch: (-want, +got)\n%s", tc.name, diff)
			}
			err = stream.CloseSend()
			fatalOnErr("CloseSend", err, t)
		})
	}
}

func TestSum(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	fatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	temp := t.TempDir()
	tempfile, err := os.CreateTemp(temp, "testfile.*")
	t.Cleanup(func() { tempfile.Close() })
	fatalOnErr("os.CreateTemp", err, t)

	in, err := os.Open("./testdata/sum.data")
	fatalOnErr("os.Open", err, t)
	t.Cleanup(func() { in.Close() })

	_, err = io.Copy(tempfile, in)
	fatalOnErr("io.Copy", err, t)

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
			sendErrFunc: fatalOnErr,
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
			sendErrFunc: fatalOnErr,
			recvErrFunc: func(op string, err error, t *testing.T) {
				if !errors.Is(err, AbsolutePathError) {
					t.Fatalf("%s: err was %v, want AbsolutePathError", op, err)
				}
			},
		},
		{
			name:        "directory",
			req:         &pb.SumRequest{Filename: temp, SumType: pb.SumType_SUM_TYPE_SHA256},
			sendErrFunc: fatalOnErr,
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
			sendErrFunc: fatalOnErr,
			recvErrFunc: fatalOnErr,
		},
		{
			name:        "invalid type",
			req:         &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_SHA256 + 999},
			reply:       nil,
			sendErrFunc: fatalOnErr,
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
			sendErrFunc: fatalOnErr,
			recvErrFunc: fatalOnErr,
		},
		{
			name: "sha512_256",
			req:  &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_SHA512_256},
			reply: &pb.SumReply{
				Filename: tempfile.Name(),
				SumType:  pb.SumType_SUM_TYPE_SHA512_256,
				Sum:      "f28247cd3fb739c77014b33f3aff1e48e7dc3674c46c10498dc8d25f4b3405a1",
			},
			sendErrFunc: fatalOnErr,
			recvErrFunc: fatalOnErr,
		},
		{
			name: "md5",
			req:  &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_MD5},
			reply: &pb.SumReply{
				Filename: tempfile.Name(),
				SumType:  pb.SumType_SUM_TYPE_MD5,
				Sum:      "485032cb71937bed2d371731498d20d3",
			},
			sendErrFunc: fatalOnErr,
			recvErrFunc: fatalOnErr,
		},
		{
			name: "crc32_ieee",
			req:  &pb.SumRequest{Filename: tempfile.Name(), SumType: pb.SumType_SUM_TYPE_CRC32IEEE},
			reply: &pb.SumReply{
				Filename: tempfile.Name(),
				SumType:  pb.SumType_SUM_TYPE_CRC32IEEE,
				Sum:      "01df4a25",
			},
			sendErrFunc: fatalOnErr,
			recvErrFunc: fatalOnErr,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := pb.NewLocalFileClient(conn)
			stream, err := client.Sum(ctx)
			fatalOnErr("client.Sum", err, t)
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
			fatalOnErr("CloseSend", err, t)
		})
	}
}
