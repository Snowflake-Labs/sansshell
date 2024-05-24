//go:build linux
// +build linux

/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

package server

import (
	"io"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
)

const (
	// FS_IMMUTABLE_FL is the flag which masks immutable state.
	// While x/sys/unix exposes a lot this particular flag isn't
	// mapped. It's the one to match below for immutable state if
	// the ioctl route has to be taken.
	FS_IMMUTABLE_FL = int(0x00000010)

	// FS_FL_USER_MODIFIABLE is the flag mask of flags that are user modifiable.
	// The ioctl can return more when querying but setting should mask with this first.
	FS_FL_USER_MODIFIABLE = int(0x000380FF)
)

var (
	// Functions we can replace with fakes for testing
	inotifyInit1    = unix.InotifyInit1
	inotifyAddWatch = unix.InotifyAddWatch
	epollCreate     = unix.EpollCreate
	epollCtl        = unix.EpollCtl
	epollWait       = unix.EpollWait

	osStat = linuxOsStat
)

// osStat is the linux specific version of stat. Depending on OS version
// returning immutable bits happens in different ways.
func linuxOsStat(path string, useLstat bool) (*pb.StatReply, error) {
	resp := &pb.StatReply{
		Filename: path,
	}

	// Use stat for the base state since we want to return a stat.Mode()
	// which is the go portable representation. Using the one from statX would
	// require converting.
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
	// Linux supports stat so we can blindly convert.
	statT := stat.Sys().(*syscall.Stat_t)

	resp.Size = stat.Size()
	resp.Mode = uint32(stat.Mode())
	resp.Modtime = timestamppb.New(stat.ModTime())
	resp.Uid = statT.Uid
	resp.Gid = statT.Gid

	statFlags := unix.AT_STATX_SYNC_AS_STAT
	if useLstat {
		statFlags |= unix.AT_SYMLINK_NOFOLLOW
	}

	statx := &unix.Statx_t{}
	err = unix.Statx(0, path, statFlags, unix.STATX_ALL, statx)
	resp.Immutable = (statx.Attributes_mask & unix.STATX_ATTR_IMMUTABLE) != 0 // Can just assign now. If there was an error it gets fixed below.
	if err != nil {
		if err.(syscall.Errno) != syscall.ENOSYS {
			return nil, status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
		}
		// If it was ENOSYS this is an old enough kernel which doesn't support statx
		// as a system call. Here we call os.Stat + ioctl to get immutable state.
		attrs, err := getFlags(path)
		if err != nil {
			// If we can't get attributes just mark it as immutable=false
			resp.Immutable = false
		} else {
			resp.Immutable = (attrs & FS_IMMUTABLE_FL) != 0
		}
	}
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
func dataPrep(f *os.File) (obj interface{}, closer func(), retErr error) {
	// Set to some defaults we can't accidentally break things if
	// close was called on these fd's before initialized.
	in := &inotify{
		iFD:     -1,
		watchFD: -1,
		epoll:   -1,
		file:    f.Name(),
	}
	closer = func() {
		unix.Close(in.iFD)
		unix.Close(in.epoll)
	}
	// Setup so we can exit and cleanup easier.
	c := func() {
		if retErr != nil {
			closer()
		}
	}
	defer c()

	// Setup inotify and set a watch on our path.
	var err error
	in.iFD, err = inotifyInit1(unix.IN_NONBLOCK)
	if err != nil {
		return nil, closer, status.Errorf(codes.Internal, "can't allocate inotify fd: %v", err)

	}

	// NOTE: This is *not* a file descriptor but an internal descriptor for inotify to
	//       use when pushing data through iFD above. Do not close() on it.
	in.watchFD, err = inotifyAddWatch(in.iFD, in.file, unix.IN_MODIFY)
	if err != nil {
		return nil, closer, status.Errorf(codes.Internal, "can't setup inotify watch: %v", err)
	}

	// Initialize epoll and set it to watch for the inotify FD to return events
	// This is only needed once, not per read.
	in.epoll, err = epollCreate(1)
	if err != nil {
		return nil, closer, status.Errorf(codes.Internal, "can't create epoll: %v", err)
	}
	in.event = &unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd:     int32(in.iFD),
	}
	if err := epollCtl(in.epoll, unix.EPOLL_CTL_ADD, in.iFD, in.event); err != nil {
		return nil, closer, status.Errorf(codes.Internal, "epollctl failed: %v", err)
	}
	return in, closer, nil
}

func consumeInotifyEvents(iFD int) error {
	buffer := make([]byte, 4096)
	for {
		n, err := unix.Read(iFD, buffer)
		switch err {
		case nil:
			if n == 0 {
				return io.ErrUnexpectedEOF
			}
		case unix.EINTR: // Do nothing
		case unix.EAGAIN:
			return nil
		default:
			return err
		}
	}
}

// dataReady is the OS specific version to indicate the given
// file has more data. With Linux we use inotify to watch the file and
// assuming the file was already at EOF.
func dataReady(fd interface{}, stream pb.LocalFile_ReadServer) error {
	inotify := fd.(*inotify)

	events := make([]unix.EpollEvent, 1)

	// Loop until we either get new data or the context gets cancalled.
	for {
		if stream.Context().Err() != nil {
			return stream.Context().Err()
		}

		// Wait READ_TIMEOUT between runs to check the context.
		n, err := epollWait(inotify.epoll, events, int(ReadTimeout.Milliseconds()))
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

			err := consumeInotifyEvents(inotify.iFD)
			if err != nil {
				return status.Errorf(codes.Internal, "read error: %v", err)
			}
			break
		}
		// Nothing (timed out) so just loop
	}
	return nil
}
