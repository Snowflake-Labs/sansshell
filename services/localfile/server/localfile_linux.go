//go:build linux
// +build linux

package server

import (
	"os"
	"syscall"
	"time"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// While x/sys/unix exposes a lot this particular flag isn't
// mapped. It's the one to match below for immutable state if
// the ioctl route has to be taken.
const FS_IMMUTABLE_FL = int(0x00000010)

// osStat is the platform agnostic version which uses basic os.Stat.
// As a result immutable bits cannot be returned.
func osStat(path string) (*pb.StatReply, error) {
	resp := &pb.StatReply{
		Filename: path,
	}
	stat := &unix.Statx_t{}
	err := unix.Statx(0, path, unix.AT_STATX_SYNC_AS_STAT, unix.STATX_ALL, stat)
	if err != nil {
		if err.(syscall.Errno) != syscall.ENOSYS {
			return nil, status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
		}
		// If it was ENOSYS this is an old enough kernel which doesn't support statx
		// as a system call. Here we call os.Stat + ioctl to get our state.
		stat, err := os.Stat(path)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
		}
		// Linux supports stat so we can blindly convert.
		stat_t := stat.Sys().(*syscall.Stat_t)

		f1, err := os.Open(path)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "stat: can't open %s - %v", path, err)
		}
		attrs, err := unix.IoctlGetInt(int(f1.Fd()), unix.FS_IOC_GETFLAGS)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "stat: can't get attributes for %s - %v", path, err)
		}
		resp.Size = stat.Size()
		resp.Mode = uint32(stat.Mode())
		resp.Modtime = timestamppb.New(stat.ModTime())
		resp.Uid = stat_t.Uid
		resp.Gid = stat_t.Gid
		resp.Immutable = (attrs & FS_IMMUTABLE_FL) != 0
		return resp, nil
	}

	// Statx case:
	resp.Size = int64(stat.Size)
	resp.Mode = uint32(stat.Mode)
	resp.Modtime = timestamppb.New(time.Unix(stat.Mtime.Sec, int64(stat.Mtime.Nsec)))
	resp.Uid = stat.Uid
	resp.Gid = stat.Gid
	resp.Immutable = (stat.Attributes_mask & unix.STATX_ATTR_IMMUTABLE) != 0
	return resp, nil
}
