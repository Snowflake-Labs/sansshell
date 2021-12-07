package server

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"hash/crc32"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"

	"golang.org/x/sys/unix"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	AbsolutePathError = status.Error(codes.InvalidArgument, "filename path must be absolute and clean")

	// For testing since otherwise tests have to run as root for these.
	chown             = unix.Chown
	changeImmutableOS = changeImmutable

	// READ_TIMEOUT is how long tail should wait on a given poll call
	// before checking context.Err() and possibly looping.
	READ_TIMEOUT = 10 * time.Second
)

// This encompasses the permission plus the setuid/gid/sticky bits one
// can set on a file/directory.
const modeMask = uint32(fs.ModePerm | fs.ModeSticky | fs.ModeSetuid | fs.ModeSetgid)

// server is used to implement the gRPC server
type server struct{}

// Read returns the contents of the named file
func (s *server) Read(req *pb.ReadActionRequest, stream pb.LocalFile_ReadServer) error {
	logger := logr.FromContextOrDiscard(stream.Context())

	r := req.GetFile()
	t := req.GetTail()

	var file string
	var offset, length int64
	switch {
	case r != nil:
		file = r.Filename
		offset = r.Offset
		length = r.Length
	case t != nil:
		file = t.Filename
		offset = t.Offset
	default:
		return status.Error(codes.InvalidArgument, "must supply a ReadRequest or a TailRequest")
	}

	logger.Info("read request", "filename", file)
	if err := util.ValidPath(file); err != nil {
		return err
	}
	f, err := os.Open(file)
	if err != nil {
		return status.Errorf(codes.Internal, "can't open file %s: %v", file, err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			logger.Error(err, "file.Close()", "file", file)
		}
	}()

	// Seek forward if requested
	if offset != 0 {
		whence := 0
		// If negative we're tailing from the end so
		// negate the sign and set whence.
		if offset < 0 {
			whence = 2
		}
		if _, err := f.Seek(offset, whence); err != nil {
			return status.Errorf(codes.Internal, "can't seek for file %s: %v", file, err)
		}
	}

	max := length
	if max == 0 {
		max = math.MaxInt64
	}

	buf := make([]byte, util.StreamingChunkSize)

	reader := io.LimitReader(f, max)

	td, closer, err := dataPrep(f)
	if err != nil {
		return err
	}
	defer closer()

	for {
		n, err := reader.Read(buf)
		// If we got EOF we're done for normal reads and wait for tails.
		if err == io.EOF {
			// If we're not tailing then we're done.
			if r != nil {
				break
			}
			if err := dataReady(td, stream); err != nil {
				return err
			}
			continue
		}

		if err != nil {
			return status.Errorf(codes.Internal, "can't read file %s: %v", file, err)
		}

		// Only send over the number of bytes we actually read or
		// else we'll send over garbage in the last packet potentially.
		if err := stream.Send(&pb.ReadReply{Contents: buf[:n]}); err != nil {
			return status.Errorf(codes.Internal, "can't send on stream for file %s: %v", file, err)
		}

		// If we got back less than a full chunk we're done for non-tail cases.
		if n < util.StreamingChunkSize {
			if r != nil {
				break
			}
		}
	}
	return nil
}

func (s *server) Stat(stream pb.LocalFile_StatServer) error {
	logger := logr.FromContextOrDiscard(stream.Context())
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "stat: recv error %v", err)
		}

		logger.Info("stat", "filename", req.Filename)
		if err := util.ValidPath(req.Filename); err != nil {
			return AbsolutePathError
		}
		resp, err := osStat(req.Filename)
		if err != nil {
			return err
		}
		if err := stream.Send(resp); err != nil {
			return status.Errorf(codes.Internal, "stat: send error %v", err)
		}
	}
}

func (s *server) Sum(stream pb.LocalFile_SumServer) error {
	logger := logr.FromContextOrDiscard(stream.Context())
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "sum: recv error %v", err)
		}
		logger.Info("sum request", "file", req.Filename, "sumtype", req.SumType.String())
		if err := util.ValidPath(req.Filename); err != nil {
			return AbsolutePathError
		}
		out := &pb.SumReply{
			SumType:  req.SumType,
			Filename: req.Filename,
		}
		var hasher hash.Hash
		switch req.SumType {
		// default to sha256 for unspecified
		case pb.SumType_SUM_TYPE_UNKNOWN, pb.SumType_SUM_TYPE_SHA256:
			hasher = sha256.New()
			out.SumType = pb.SumType_SUM_TYPE_SHA256
		case pb.SumType_SUM_TYPE_MD5:
			hasher = md5.New()
		case pb.SumType_SUM_TYPE_SHA512_256:
			hasher = sha512.New512_256()
		case pb.SumType_SUM_TYPE_CRC32IEEE:
			hasher = crc32.NewIEEE()
		default:
			return status.Errorf(codes.InvalidArgument, "invalid sum type value %d", req.SumType)
		}
		if err := func() error {
			f, err := os.Open(req.Filename)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(hasher, f); err != nil {
				logger.Error(err, "io.Copy", "file", req.Filename)
				return status.Errorf(codes.Internal, "copy/read error: %v", err)
			}
			out.Sum = hex.EncodeToString(hasher.Sum(nil))
			return nil
		}(); err != nil {
			return status.Errorf(codes.Internal, "can't create sum: %v", err)
		}
		if err := stream.Send(out); err != nil {
			return status.Errorf(codes.Internal, "sum: send error %v", err)
		}
	}
}

func setupOutput(a *pb.FileAttributes) (*os.File, *immutableState, error) {
	// Validate path. We'll go ahead and write the data to a tmpfile and
	// do the overwrite check when we rename below.
	filename := a.Filename
	if err := util.ValidPath(filename); err != nil {
		return nil, nil, err
	}

	f, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "can't create tmp file: %v", err)
	}

	// Set owner/gid/perms now since we have an open FD to the file and we don't want
	// to accidentally leave this in another otherwise default state.
	// Except we don't trigger immutable now or we won't be able to write to it.
	immutable, err := validateAndSetAttrs(f.Name(), a.Attributes, false)
	return f, immutable, err
}

func finalizeFile(d *pb.FileWrite, f *os.File, filename string, immutable *immutableState) error {
	if err := f.Close(); err != nil {
		return status.Errorf(codes.Internal, "error closing %s - %v", f.Name(), err)
	}

	// Do one final check (though racy) to see if the file exists.
	_, err := os.Stat(filename)
	if err == nil && !d.Overwrite {
		return status.Errorf(codes.Internal, "file %s exists and overwrite set to false", filename)
	}

	// Rename tmp file to real destination.
	if err := os.Rename(f.Name(), filename); err != nil {
		return status.Errorf(codes.Internal, "error renaming %s -> %s - %v", f.Name(), filename, err)
	}

	// Now set immutable if requested.
	if immutable.setImmutable && immutable.immutable {
		if err := changeImmutableOS(filename, immutable.immutable); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) Write(stream pb.LocalFile_WriteServer) (retErr error) {
	logger := logr.FromContextOrDiscard(stream.Context())

	var f *os.File
	var d *pb.FileWrite
	cleanup := func() {
		if retErr != nil {
			if f != nil {
				// Close and then remove the tmpfile since some error happened.
				f.Close()
				os.Remove(f.Name())
			}
		}
	}
	defer cleanup()

	first := false
	var immutable *immutableState
	var filename string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "write: recv error %v", err)
		}

		switch {
		case req.GetDescription() != nil:
			if first {
				return status.Errorf(codes.InvalidArgument, "can't send multiple description blocks")
			}
			first = true
			d = req.GetDescription()
			a := d.GetAttrs()
			if a == nil {
				return status.Errorf(codes.InvalidArgument, "must send attrs")
			}
			filename = a.Filename
			logger.Info("write file", filename)
			f, immutable, err = setupOutput(a)
			if err != nil {
				return err
			}
		case req.GetContents() != nil:
			if !first {
				return status.Errorf(codes.InvalidArgument, "can't send content before description block")
			}
			n, err := f.Write(req.GetContents())
			if err != nil {
				return status.Errorf(codes.Internal, "write error: %v", err)
			}
			if got, want := n, len(req.GetContents()); got != want {
				return status.Errorf(codes.Internal, "short write. expected %d but only wrote %d", got, want)
			}
		default:
			return status.Error(codes.InvalidArgument, "Must supply either a Description or Contents")
		}
	}

	// Finalize to the final destination and possibly set immutable.
	if err := finalizeFile(d, f, filename, immutable); err != nil {
		return err
	}
	return nil
}

func (s *server) Copy(ctx context.Context, req *pb.CopyRequest) (_ *emptypb.Empty, retErr error) {
	logger := logr.FromContextOrDiscard(ctx)

	if req.GetDestination() == nil {
		return nil, status.Error(codes.InvalidArgument, "destination must be filled in")
	}
	d := req.GetDestination()

	if req.Bucket == "" {
		return nil, status.Error(codes.InvalidArgument, "bucket must be filled in")
	}
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key must be filled in")
	}

	a := d.GetAttrs()
	if a == nil {
		return nil, status.Errorf(codes.InvalidArgument, "must send attrs")
	}
	filename := a.Filename
	logger.Info("copy file", filename)
	f, immutable, err := setupOutput(a)
	if err != nil {
		return nil, err
	}
	cleanup := func() {
		if retErr != nil {
			if f != nil {
				// Close and then remove the tmpfile since some error happened.
				f.Close()
				os.Remove(f.Name())
			}
		}
	}
	defer cleanup()

	// Copy file over
	b, err := blob.OpenBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't open bucket %s - %v", req.Bucket, err)
	}
	reader, err := b.NewReader(ctx, req.Key, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't open key %s in bucket %s - %v", req.Key, req.Bucket, err)
	}
	written, err := io.Copy(f, reader)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't copy from bucket %s/%s Only wrote %d bytes - %v", req.Bucket, req.Key, written, err)
	}
	if err := b.Close(); err != nil {
		return nil, status.Errorf(codes.Internal, "error closing bucket %s - %v", req.Bucket, err)
	}

	// Finalize to the final destination and possibly set immutable.
	if err := finalizeFile(d, f, filename, immutable); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *server) List(req *pb.ListRequest, server pb.LocalFile_ListServer) error {
	logger := logr.FromContextOrDiscard(server.Context())
	if req.Entry == "" {
		return status.Errorf(codes.InvalidArgument, "filename must be filled in")
	}
	if err := util.ValidPath(req.Entry); err != nil {
		return err
	}

	// We always send back the entry first.
	logger.Info("ls", "filename", req.Entry)
	resp, err := osStat(req.Entry)
	if err != nil {
		return err
	}
	if err := server.Send(&pb.ListReply{Entry: resp}); err != nil {
		return status.Errorf(codes.Internal, "list: send error %v", err)
	}

	// If it's directory we'll open it and go over its entries.
	if fs.FileMode(resp.Mode).IsDir() {
		entries, err := os.ReadDir(req.Entry)
		if err != nil {
			return status.Errorf(codes.Internal, "readdir: %v", err)
		}
		// Only do one level so iterate these and we're done.
		for _, e := range entries {
			name := filepath.Join(req.Entry, e.Name())
			logger.Info("ls", "filename", name)
			resp, err := osStat(name)
			if err != nil {
				return err
			}
			if err := server.Send(&pb.ListReply{Entry: resp}); err != nil {
				return status.Errorf(codes.Internal, "list: send error %v", err)
			}
		}
	}
	return nil
}

// immutableState tracks the parsed state of immutable from the slice of FileAttribute.
type immutableState struct {
	setImmutable bool // Whether immutable was set (so we should change state).
	immutable    bool // The immutable value (only applies if setImmutable is true).
}

func validateAndSetAttrs(filename string, attrs []*pb.FileAttribute, doImmutable bool) (*immutableState, error) {
	uid, gid := int(-1), int(-1)
	setMode, setImmutable, immutable := false, false, false
	mode := uint32(0)

	for _, attr := range attrs {
		switch a := attr.Value.(type) {
		case *pb.FileAttribute_Uid:
			if uid != -1 {
				return nil, status.Error(codes.InvalidArgument, "cannot set uid more than once")
			}
			uid = int(a.Uid)
		case *pb.FileAttribute_Gid:
			if gid != -1 {
				return nil, status.Error(codes.InvalidArgument, "cannot set gid more than once")
			}
			gid = int(a.Gid)
		case *pb.FileAttribute_Mode:
			if setMode {
				return nil, status.Error(codes.InvalidArgument, "cannot set mode more than once")
			}
			mode = a.Mode
			setMode = true
		case *pb.FileAttribute_Immutable:
			if setImmutable {
				return nil, status.Error(codes.InvalidArgument, "cannot set immutable more than once")
			}
			immutable = a.Immutable
			setImmutable = true
		}
	}

	if uid != -1 || gid != -1 {
		if err := chown(filename, uid, gid); err != nil {
			return nil, status.Errorf(codes.Internal, "error from chown: %v", err)
		}
	}

	if setMode {
		if err := unix.Chmod(filename, mode&modeMask); err != nil {
			return nil, status.Errorf(codes.Internal, "error from chmod: %v", err)
		}
	}

	if doImmutable && setImmutable {
		if err := changeImmutableOS(filename, immutable); err != nil {
			return nil, err
		}
	}
	return &immutableState{
		setImmutable: setImmutable,
		immutable:    immutable,
	}, nil
}

func (s *server) SetFileAttributes(ctx context.Context, req *pb.SetFileAttributesRequest) (*emptypb.Empty, error) {
	if req.Attrs == nil {
		return nil, status.Error(codes.InvalidArgument, "attrs must be filled in")
	}
	p := req.Attrs.Filename
	if p == "" {
		return nil, status.Error(codes.InvalidArgument, "filename must be filled in")
	}
	if err := util.ValidPath(p); err != nil {
		return nil, err
	}

	// Don't care about immutable state as we set it if it came across.
	if _, err := validateAndSetAttrs(p, req.Attrs.Attributes, true); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterLocalFileServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
