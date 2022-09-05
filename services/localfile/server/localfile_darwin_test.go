//go:build darwin
// +build darwin

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

func TestTailDarwin(t *testing.T) {
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

	failKqueue := func() (fd int, err error) {
		return 0, errors.New("fail")
	}

	// The 3 states we might have kevent fail around.
	failRegister := false
	failUpdate := false
	badResult := false // Return something other than EVFILT_VNODE

	failKevent := func(kq int, changes, events []unix.Kevent_t, timeout *unix.Timespec) (n int, err error) {
		if len(changes) > 0 && failRegister {
			return 0, errors.New("fail register")
		}
		if len(events) > 0 {
			if failUpdate {
				return 0, errors.New("fail update")
			}
			if badResult {
				events[0].Filter = unix.EVFILT_EXCEPT
			}
		}
		return 1, nil
	}

	for _, tc := range []struct {
		name         string
		kqueue       func() (int, error)
		kevent       func(int, []unix.Kevent_t, []unix.Kevent_t, *unix.Timespec) (int, error)
		failRegister bool
		failUpdate   bool
		badResult    bool
		stopRecv     bool // Stop after one recv since it'll just keep failing
	}{
		{
			name:     "kqueue fail",
			kqueue:   failKqueue,
			stopRecv: true,
		},
		{
			name:         "kevent register",
			kevent:       failKevent,
			failRegister: true,
		},
		{
			name:       "kevent update",
			kevent:     failKevent,
			failUpdate: true,
		},
		{
			name:      "kevent bad data",
			kevent:    failKevent,
			badResult: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			oKqueue := kqueue
			oKevent := kevent
			t.Cleanup(func() {
				kqueue = oKqueue
				kevent = oKevent
				failRegister = false
				failUpdate = false
				badResult = false
			})

			if tc.kqueue != nil {
				kqueue = tc.kqueue
			}
			if tc.kevent != nil {
				kevent = tc.kevent
			}
			failRegister = tc.failRegister
			failUpdate = tc.failUpdate
			badResult = tc.badResult

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
