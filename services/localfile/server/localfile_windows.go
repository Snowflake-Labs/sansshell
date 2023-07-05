package server

import (
	"io/fs"
	"math"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
)

var (
	osAgnosticRm    = os.Remove
	osAgnosticRmdir = os.Remove
	osStat          = windowsOsStat
)

// windowsOsStat is the Windows version of geting file status. We only support
// the Windows-relevant subset of information.
func windowsOsStat(path string, useLstat bool) (*pb.StatReply, error) {
	var stat fs.FileInfo
	var err error
	if useLstat {
		stat, err = os.Lstat(path)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "stat: os.Lstat error %v", err)
		}
	} else {
		stat, err = os.Stat(path)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
		}
	}
	resp := &pb.StatReply{
		Filename: path,
		Size:     stat.Size(),
		Mode:     uint32(stat.Mode()),
		Modtime:  timestamppb.New(stat.ModTime()),
		Uid:      math.MaxUint32,
		Gid:      math.MaxUint32,
	}
	return resp, nil
}

// Windows doesn't have immutable files
func changeImmutable(path string, immutable bool) error {
	return status.Error(codes.Unimplemented, "immutable not supported")
}

func dataPrep(f *os.File) (interface{}, func(), error) {
	return nil, func() {}, nil
}

// dataReady is the OS specific version to indicate the given
// file has more data. This could be optimized for Windows with
// ReadDirectoryChangesW and golang.org/x/sys/windows.
func dataReady(_ interface{}, stream pb.LocalFile_ReadServer) error {
	if stream.Context().Err() != nil {
		return stream.Context().Err()
	}
	time.Sleep(ReadTimeout)
	// Time to try again.
	return nil
}
