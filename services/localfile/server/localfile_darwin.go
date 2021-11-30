//go:build darwin
// +build darwin

package server

import (
	"os"
	"syscall"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SF_IMMUTABLE is the system immutable flag.
// Since while Stat_t imports Flags correctly it doesn't bother
// to map them from sys/stat.h
const SF_IMMUTABLE = uint32(0x00020000)

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
