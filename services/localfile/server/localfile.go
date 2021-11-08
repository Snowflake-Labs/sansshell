package server

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"hash/crc32"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	AbsolutePathError = status.Error(codes.InvalidArgument, "filename path must be absolute")
)

// server is used to implement the gRPC server
type server struct{}

// Read returns the contents of the named file
func (s *server) Read(req *pb.ReadRequest, stream pb.LocalFile_ReadServer) error {
	log.Printf("Received request for LocalFile.Read: %+v", req)

	file := req.Filename
	log.Printf("Received request for: %v", file)
	if !filepath.IsAbs(file) {
		return AbsolutePathError
	}
	f, err := os.Open(file)
	if err != nil {
		return status.Errorf(codes.Internal, "can't open file %s: %v", file, err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Can't close file %s: %v", file, err)
		}
	}()

	// Seek forward if requested
	if s := req.GetOffset(); s != 0 {
		whence := 0
		// If negative we're tailing from the end so
		// negate the sign and set whence.
		if s < 0 {
			whence = 2
		}
		if _, err := f.Seek(s, whence); err != nil {
			return status.Errorf(codes.Internal, "can't seek for file %s: %v", file, err)
		}
	}

	max := req.GetLength()
	if max == 0 {
		max = math.MaxInt64
	}

	buf := make([]byte, util.StreamingChunkSize)

	reader := io.LimitReader(f, max)

	for {
		n, err := reader.Read(buf)

		// If we got EOF we're done.
		if err == io.EOF {
			break
		}

		if err != nil {
			return status.Errorf(codes.Internal, "can't read file %s: %v", file, err)
		}

		// Only send over the number of bytes we actually read or
		// else we'll send over garbage in the last packet potentially.
		if err := stream.Send(&pb.ReadReply{Contents: buf[:n]}); err != nil {
			return status.Errorf(codes.Internal, "can't send on stream for file %s: %v", file, err)
		}

		// If we got back less than a full chunk we're done.
		if n < util.StreamingChunkSize {
			break
		}
	}
	return nil
}

func (s *server) Stat(stream pb.LocalFile_StatServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("stat: recv error %v", err)
			return status.Errorf(codes.Internal, "stat: recv error %v", err)
		}
		log.Printf("Received request for LocalFile.Stat: %+v", req)

		log.Printf("stat: request for file: %v", req.Filename)
		if !filepath.IsAbs(req.Filename) {
			return AbsolutePathError
		}
		stat, err := os.Stat(req.Filename)
		if err != nil {
			return status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
		}
		stat_t, ok := stat.Sys().(*syscall.Stat_t)
		if !ok || stat_t == nil {
			return status.Error(codes.Unimplemented, "stat not supported")
		}
		if err := stream.Send(&pb.StatReply{
			Filename: req.Filename,
			Size:     stat.Size(),
			Mode:     uint32(stat.Mode()),
			Modtime:  timestamppb.New(stat.ModTime()),
			Uid:      stat_t.Uid,
			Gid:      stat_t.Gid,
		}); err != nil {
			log.Printf("stat: send error %v", err)
			return status.Errorf(codes.Internal, "stat: send error %v", err)
		}
	}
}

func (s *server) Sum(stream pb.LocalFile_SumServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("sum: recv error %v", err)
			return status.Errorf(codes.Internal, "sum: recv error %v", err)
		}
		log.Printf("Received request for Localfile.Sum: %+v", req)
		if !filepath.IsAbs(req.Filename) {
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
			log.Printf("sum: invalid sum type %v", req.SumType)
			return status.Errorf(codes.InvalidArgument, "invalid sum type value %d", req.SumType)
		}
		if err := func() error {
			f, err := os.Open(req.Filename)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(hasher, f); err != nil {
				log.Printf("sum: copy error %v", err)
				return err
			}
			out.Sum = hex.EncodeToString(hasher.Sum(nil))
			return nil
		}(); err != nil {
			return status.Errorf(codes.Internal, "can't create sum: %v", err)
		}
		if err := stream.Send(out); err != nil {
			log.Printf("sum: send error %v", err)
			return status.Errorf(codes.Internal, "sum: send error %v", err)
		}
	}
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterLocalFileServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
