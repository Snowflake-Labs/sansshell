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

const (
	// While x/sys/unix exposes a lot this particular flag isn't
	// mapped. It's the one to match below for immutable state if
	// the ioctl route has to be taken.
	FS_IMMUTABLE_FL = int(0x00000010)

	// The mask of flags that are user modifiable. The ioctl can return
	// more when querying but setting should mask with this first.
	FS_FL_USER_MODIFIABLE = int(0x000380FF)
)

// osStat is the linux specific version of stat. Depending on OS version
// returning immutable bits happens in different ways.
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

		attrs, err := getFlags(path)
		if err != nil {
			return nil, err
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

func getFlags(path string) (int, error) {
	f1, err := os.Open(path)
	if err != nil {
		return 0, status.Errorf(codes.Internal, "stat: can't open %s - %v", path, err)
	}
	defer func() { f1.Close() }()

	attrs, err := unix.IoctlGetInt(int(f1.Fd()), unix.FS_IOC_GETFLAGS)
	if err != nil {
		return 0, status.Errorf(codes.Internal, "stat: can't get attributes for %s - %v", path, err)
	}
	return attrs, nil
}

// changeImmutable is the Linux specific implementation for changing
// the immutable bit.
func changeImmutable(path string, immutable bool) error {
	attrs, err := getFlags(path)
	if err != nil {
		return err
	}
	// Need to make off to only the ones we can set.
	attrs &= FS_FL_USER_MODIFIABLE

	// Clear immutable and then possibly set it.
	attrs &= ^FS_IMMUTABLE_FL
	if immutable {
		attrs |= FS_IMMUTABLE_FL
	}

	f1, err := os.Open(path)
	if err != nil {
		return status.Errorf(codes.Internal, "stat: can't open %s - %v", path, err)
	}
	defer func() { f1.Close() }()
	return unix.IoctlSetPointerInt(int(f1.Fd()), unix.FS_IOC_SETFLAGS, attrs)
}

type inotify struct {
	iFD     int
	watchFD int
	epoll   int
	file    string
	event   *unix.EpollEvent
}

// dataPrep should be called before entering a loop watching a file.
// It returns an opaque object to pass to dataReady() and a function
// which should be run on exit (i.e. defer it).
func dataPrep(f *os.File) (interface{}, func(), error) {
	// Set to some defaults we can't accidentally break things if
	// close was called on these fd's before initialized.
	in := &inotify{
		iFD:     -1,
		watchFD: -1,
		epoll:   -1,
		file:    f.Name(),
	}
	closer := func() {
		unix.Close(in.iFD)
		unix.Close(in.epoll)
	}

	// Setup inotify and set a watch on our path.
	var err error
	in.iFD, err = unix.InotifyInit1(0)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "can't allocate inotify fd: %v", err)
	}

	// NOTE: This is *not* a file descriptor but an internal descriptor for inotify to
	//       use when pushing data through iFD above. Do not close() on it.
	in.watchFD, err = unix.InotifyAddWatch(in.iFD, in.file, unix.IN_MODIFY)
	if err != nil {
		closer()
		return nil, nil, status.Errorf(codes.Internal, "can't setup inotify watch: %v", err)
	}

	// Initialize epoll and set it to watch for the inotify FD to return events
	// This is only needed once, not per read.
	in.epoll, err = unix.EpollCreate(1)
	if err != nil {
		closer()
		return nil, nil, status.Errorf(codes.Internal, "can't create epoll: %v", err)
	}
	in.event = &unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd:     int32(in.iFD),
	}
	if err := unix.EpollCtl(in.epoll, unix.EPOLL_CTL_ADD, in.iFD, in.event); err != nil {
		closer()
		return nil, nil, status.Errorf(codes.Internal, "epollctl failed: %v", err)
	}
	return in, closer, nil
}

// dataReady is the OS specific version to indicate the given
// file has more data. With Linux we use inotify to watch the file and
// assuming the file was already at EOF.
func dataReady(fd interface{}, stream pb.LocalFile_ReadServer) error {
	inotify, ok := fd.(*inotify)
	if !ok {
		return status.Errorf(codes.Internal, "invalid type passed. Expected *inotify and got %T", fd)
	}

	events := make([]unix.EpollEvent, 1)

	// Loop until we either get new data or the context gets cancalled.
	for {
		if stream.Context().Err() != nil {
			return stream.Context().Err()
		}

		// Wait READ_TIMEOUT between runs to check the context.
		n, err := unix.EpollWait(inotify.epoll, events, int(READ_TIMEOUT.Milliseconds()))
		if err != nil {
			// If we got EINTR we can just loop again.
			if err == unix.EINTR {
				continue
			}
			return status.Errorf(codes.Internal, "epoll error: %v", err)
		}
		if n == 1 {
			if events[0].Fd != int32(inotify.iFD) {
				return status.Errorf(codes.Internal, "epoll event for wrong FD? got %+v", events[0])
			}
			// In theory we should read the event from the inotify FD and interpret it but we're only
			// monitoring one thing and there's no errors that report in-band (just via errno).
			break
		}
		// Nothing (timed out) so just loop
	}
	return nil
}
