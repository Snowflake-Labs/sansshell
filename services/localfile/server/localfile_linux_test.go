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
	"context"
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func TestTailLinux(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	// Create a file with some initial data.
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("can't create tmpfile", err, t)
	data := "Some data\n"
	_, err = f1.WriteString(data)
	testutil.FatalOnErr("WriteString", err, t)
	name := f1.Name()
	err = f1.Close()
	testutil.FatalOnErr("closing file", err, t)

	client := pb.NewLocalFileClient(conn)

	fakeInotifyInit1 := func(int) (int, error) {
		return -1, errors.New("fail inotify init")
	}
	fakeInotifyAddWatch := func(int, string, uint32) (int, error) {
		return -1, errors.New("fail inotify add watch")
	}
	fakeEpollCreate := func(int) (int, error) {
		return -1, errors.New("fail epoll create")
	}
	fakeEpollCtl := func(int, int, int, *unix.EpollEvent) error {
		return errors.New("fail epollctl")
	}

	// The 2 states we might have EpollWait fail around.
	failEpollWaitEINTR := false
	failEpollWaitError := false // general failure
	failEpollWaitBadFD := false

	failCount := 0
	fakeEpollWait := func(epfd int, events []unix.EpollEvent, msec int) (int, error) {
		// We only do this once and then drop through below and fail normally.
		if failEpollWaitEINTR && failCount == 0 {
			failCount++
			return 0, unix.EINTR
		}
		if failEpollWaitError || failEpollWaitEINTR {
			return 0, errors.New("epoll wait")
		}
		if failEpollWaitBadFD {
			events[0].Fd = -1
			return 1, nil
		}
		return 0, nil
	}

	for _, tc := range []struct {
		name               string
		inotifyInit1       func(int) (int, error)
		inotifyAddWatch    func(int, string, uint32) (int, error)
		epollCreate        func(int) (int, error)
		epollCtl           func(int, int, int, *unix.EpollEvent) error
		epollWait          func(int, []unix.EpollEvent, int) (int, error)
		failEpollWaitEINTR bool
		failEpollWaitError bool
		failEpollWaitBadFD bool
		stopRecv           bool
	}{
		{
			name:         "inotify init fail",
			inotifyInit1: fakeInotifyInit1,
			stopRecv:     true,
		},
		{
			name:            "inotify add watch fail",
			inotifyAddWatch: fakeInotifyAddWatch,
			stopRecv:        true,
		},
		{
			name:        "epoll create",
			epollCreate: fakeEpollCreate,
			stopRecv:    true,
		},
		{
			name:     "epoll ctl",
			epollCtl: fakeEpollCtl,
			stopRecv: true,
		},
		{
			name:               "epollwait general failure",
			epollWait:          fakeEpollWait,
			failEpollWaitError: true,
		},
		{
			name:               "epollwait EINTR failure",
			epollWait:          fakeEpollWait,
			failEpollWaitEINTR: true},
		{
			name:               "epollwait bad fd",
			epollWait:          fakeEpollWait,
			failEpollWaitBadFD: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			oInotifyInit1 := inotifyInit1
			oInotifyAddWatch := inotifyAddWatch
			oEpollCreate := epollCreate
			oEpollCtl := epollCtl
			oEpollWait := epollWait
			failCount = 0
			t.Cleanup(func() {
				inotifyInit1 = oInotifyInit1
				inotifyAddWatch = oInotifyAddWatch
				epollCreate = oEpollCreate
				epollCtl = oEpollCtl
				epollWait = oEpollWait
				failEpollWaitEINTR = false
				failEpollWaitError = false
				failEpollWaitBadFD = false
			})

			if tc.inotifyInit1 != nil {
				inotifyInit1 = tc.inotifyInit1
			}
			if tc.inotifyAddWatch != nil {
				inotifyAddWatch = tc.inotifyAddWatch
			}
			if tc.epollCreate != nil {
				epollCreate = tc.epollCreate
			}
			if tc.epollCtl != nil {
				epollCtl = tc.epollCtl
			}
			if tc.epollWait != nil {
				epollWait = tc.epollWait
			}
			failEpollWaitEINTR = tc.failEpollWaitEINTR
			failEpollWaitError = tc.failEpollWaitError
			failEpollWaitBadFD = tc.failEpollWaitBadFD

			stream, err := client.Read(ctx, &pb.ReadActionRequest{
				Request: &pb.ReadActionRequest_Tail{
					Tail: &pb.TailRequest{
						Filename: name,
					},
				},
			})
			testutil.FatalOnErr("setup for "+tc.name, err, t)
			_, err = stream.Recv()
			t.Log(err)
			if tc.stopRecv {
				testutil.FatalOnNoErr(tc.name+" should fail", err, t)
				return
			}
			testutil.FatalOnErr(tc.name+" - first recv", err, t)
			// This one should fail as we try and stat/wait on file changes
			_, err = stream.Recv()
			t.Log(err)
			testutil.FatalOnNoErr(tc.name+" should fail", err, t)
		})
	}
}
