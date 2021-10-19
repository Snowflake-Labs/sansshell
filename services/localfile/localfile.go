package localfile

// To regenerate the proto headers if the .proto changes, just run go generate
// and this encodes the necessary magic:
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative localfile.proto

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

// TODO(jchacon): Make this configurable
// The chunk size we use when sending replies to Read on
// the stream.
var chunkSize = 128 * 1024

// Read returns the contents of the named file
func (s *server) Read(in *ReadRequest, stream LocalFile_ReadServer) error {
	file := in.GetFilename()
	log.Printf("Received request for: %v", file)
	if !filepath.IsAbs(file) {
		return AbsolutePathError
	}
	f, err := os.Open(file)
	if err != nil {
		log.Printf("Can't open file %s: %v", file, err)
		return err
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Can't close file %s: %v", file, err)
		}
	}()

	// Seek forward if requested
	if s := in.GetOffset(); s != 0 {
		whence := 0
		// If negative we're tailing from the end so
		// negate the sign and set whence.
		if s < 0 {
			whence = 2
		}
		if _, err := f.Seek(s, whence); err != nil {
			return err
		}
	}

	max := in.GetLength()
	if max == 0 {
		max = math.MaxInt64
	}

	buf := make([]byte, chunkSize)

	reader := io.LimitReader(f, max)

	for {
		n, err := reader.Read(buf)

		// If we got EOF we're done.
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		// Only send over the number of bytes we actually read or
		// else we'll send over garbage in the last packet potentially.
		if err := stream.Send(&ReadReply{Contents: buf[:n]}); err != nil {
			return err
		}

		// If we got back less than a full chunk we're done.
		if n < chunkSize {
			break
		}
	}
	return nil
}

func (s *server) Stat(stream LocalFile_StatServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("stat: recv error %v", err)
			return err
		}
		log.Printf("stat: request for file: %v", in.Filename)
		if !filepath.IsAbs(in.Filename) {
			return AbsolutePathError
		}
		stat, err := os.Stat(in.Filename)
		if err != nil {
			log.Printf("stat: os.Stat error %v", err)
			return err
		}
		stat_t, ok := stat.Sys().(*syscall.Stat_t)
		if !ok || stat_t == nil {
			return status.Error(codes.Unimplemented, "stat not supported")
		}
		if err := stream.Send(&StatReply{
			Filename: in.Filename,
			Size:     stat.Size(),
			Mode:     uint32(stat.Mode()),
			Modtime:  timestamppb.New(stat.ModTime()),
			Uid:      stat_t.Uid,
			Gid:      stat_t.Gid,
		}); err != nil {
			log.Printf("stat: send error %v", err)
			return err
		}
	}
}

func (s *server) Sum(stream LocalFile_SumServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("sum: recv error %v", err)
			return err
		}
		log.Printf("sum: request for file: %v", in.Filename)
		if !filepath.IsAbs(in.Filename) {
			return AbsolutePathError
		}
		out := &SumReply{
			SumType:  in.SumType,
			Filename: in.Filename,
		}
		var hasher hash.Hash
		switch in.SumType {
		// default to sha256 for unspecified
		case SumType_SUM_TYPE_UNKNOWN, SumType_SUM_TYPE_SHA256:
			hasher = sha256.New()
			out.SumType = SumType_SUM_TYPE_SHA256
		case SumType_SUM_TYPE_MD5:
			hasher = md5.New()
		case SumType_SUM_TYPE_SHA512_256:
			hasher = sha512.New512_256()
		case SumType_SUM_TYPE_CRC32IEEE:
			hasher = crc32.NewIEEE()
		default:
			log.Printf("sum: invalid sum type %v", in.SumType)
			return status.Errorf(codes.InvalidArgument, "invalid sum type value %d", in.SumType)
		}
		if err := func() error {
			f, err := os.Open(in.Filename)
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
			return err
		}
		if err := stream.Send(out); err != nil {
			log.Printf("sum: send error %v", err)
			return err
		}
	}
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	RegisterLocalFileServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
