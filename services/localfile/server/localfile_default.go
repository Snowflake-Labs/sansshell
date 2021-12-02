//go:build !(linux || darwin)
// +build !linux,!darwin

package server

import (
	"os"
	"syscall"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// osStat is the platform agnostic version which uses basic os.Stat.
// As a result immutable bits cannot be returned.
func osStat(path string) (*pb.StatReply, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
	}
	// If a system doesn't support this an OS specific version of osStat needs to be
	// written which simulates stat().
	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok || stat_t == nil {
		return nil, status.Error(codes.Unimplemented, "stat not supported")
	}
	resp := &pb.StatReply{
		Filename: path,
		Size:     stat.Size(),
		Mode:     uint32(stat.Mode()),
		Modtime:  timestamppb.New(stat.ModTime()),
		Uid:      stat_t.Uid,
		Gid:      stat_t.Gid,
	}
	return resp, nil
}

// changeImmutable is the default implementation for changing
// immutable bits (which is unsupported).
func changeImmutable(path string, immutable bool) error {
	return status.Error(codes.Unimplemented, "immutable not supported")
}
