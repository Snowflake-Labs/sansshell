//go:build darwin
// +build darwin

package server

import (
	"os"
	"syscall"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// The user and system flags.
	// While Stat_t imports Flags correctly it doesn't bother
	// to map them from sys/stat.h

	// UF_SETTABLE is the mask for user settable flags. This
	// should be combined with SF_SETTABLE before setting flags
	// based on retrieving them beforehand.
	UF_SETTABLE = uint32(0x0000FFFF)

	// SF_SETTABLE is the mask for system setting flags. This
	// should be combined with UF_SETTABLE before setting flags
	// based on retrieving them beforehand.
	SF_SETTABLE = uint32(0x3FFF0000)

	// SF_IMMUTABLE is the system immutable flag.
	SF_IMMUTABLE = uint32(0x00020000)
)

// osStat is the platform agnostic version which uses basic os.Stat.
// As a result immutable bits cannot be returned.
func osStat(path string) (*pb.StatReply, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
	}
	// Darwin supports stat so we can blindly convert.
	stat_t := stat.Sys().(*syscall.Stat_t)
	resp := &pb.StatReply{
		Filename:  path,
		Size:      stat.Size(),
		Mode:      uint32(stat.Mode()),
		Modtime:   timestamppb.New(stat.ModTime()),
		Uid:       stat_t.Uid,
		Gid:       stat_t.Gid,
		Immutable: (stat_t.Flags & SF_IMMUTABLE) != 0,
	}
	return resp, nil
}

// changeImmutable is the Darwin specific implementation for changing
// the immutable (system only) bit.
func changeImmutable(path string, immutable bool) error {
	stat, err := os.Stat(path)
	if err != nil {
		return status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
	}
	// Darwin supports stat so we can blindly convert.
	stat_t := stat.Sys().(*syscall.Stat_t)
	// Mask to what we can set, turn off immutable and then
	// possibly back on.
	flags := stat_t.Flags & (UF_SETTABLE | SF_SETTABLE)
	flags &= ^SF_IMMUTABLE
	if immutable {
		flags |= SF_IMMUTABLE
	}
	return unix.Chflags(path, int(flags))
}
