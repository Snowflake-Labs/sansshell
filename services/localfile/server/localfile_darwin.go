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

type kqueueFile struct {
	path   string
	fileFD uintptr
	kqFD   int
}

// dataPrep should be called before entering a loop watching a file.
// It returns an opaque object to pass to dataReady() and a function
// which should be run on exit (i.e. defer it).
func dataPrep(f *os.File) (interface{}, func(), error) {
	// Set to some defaults we can't accidentally break things if
	// close was called on these fd's before initialized.
	kq := &kqueueFile{
		path:   f.Name(),
		fileFD: f.Fd(),
		kqFD:   -1,
	}
	var err error
	// Initialize kqueue and setup for cleaning it up later.
	kq.kqFD, err = unix.Kqueue()
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "can't allocate kqueue: %v", err)
	}
	closer := func() {
		unix.Close(kq.kqFD)
	}
	return kq, closer, nil
}

// dataReady is the OS specific version to indicate the given
// file has more data. With Darwin we use kqueue to watch the file
// Assuming the file was already at EOF.
func dataReady(kq interface{}, stream pb.LocalFile_ReadServer) error {
	kqf, ok := kq.(*kqueueFile)
	if !ok {
		return status.Errorf(codes.Internal, "invalid type passed. Expected *kqueueFD and got %T", kq)
	}
	changes := make([]unix.Kevent_t, 1)
	events := make([]unix.Kevent_t, 1)

	for {
		if stream.Context().Err() != nil {
			return stream.Context().Err()
		}
		unix.SetKevent(&changes[0], int(kqf.fileFD), unix.EVFILT_VNODE, unix.EV_ADD|unix.EV_CLEAR|unix.EV_ENABLE)
		changes[0].Fflags = unix.NOTE_WRITE
		ret, err := unix.Kevent(kqf.kqFD, changes, nil, nil)
		if ret == -1 {
			return status.Errorf(codes.Internal, "can't register kqueue: %v", err)
		}

		// Wait 10s in between requests so we can check the stream context too.
		ts := &unix.Timespec{
			Sec: 10,
		}
		n, err := unix.Kevent(kqf.kqFD, nil, events, ts)
		if err != nil {
			return status.Errorf(codes.Internal, "can't get kqueue events: %v", err)
		}
		// Something happened
		if n == 1 {
			// Generally this indicates an error.
			if events[0].Filter != unix.EVFILT_VNODE {
				return status.Errorf(codes.Internal, "got incorrect kevent back: %+v", events[0])
			}
			break
		}
		// Nothing (timed out) so just loop
	}
	return nil
}
