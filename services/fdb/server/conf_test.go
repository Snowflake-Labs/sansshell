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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func TestRead(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewConfClient(conn)

	wd, err := os.Getwd()
	testutil.FatalOnErr("can't get current working directory", err, t)

	path := filepath.Join(wd, "testdata", "foundationdb.conf")

	for _, tc := range []struct {
		name      string
		req       *pb.ReadRequest
		respValue string
	}{
		{
			name:      "read cluster_file from general",
			respValue: "/etc/foundationdb/fdb.cluster",
			req: &pb.ReadRequest{
				Location: &pb.Location{
					File:    path,
					Section: "general",
					Key:     "cluster_file",
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Read(ctx, tc.req)
			testutil.FatalOnErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			if got, want := resp.Value, tc.respValue; got != want {
				t.Fatalf("response string does not match. Want %q Got %q", want, got)
			}
		})
	}

}

func TestWrite(t *testing.T) {
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("can't create tmpfile", err, t)
	_, err = f1.WriteString(`
[general]
cluster_file = /etc/foundatindb/fdb.cluster`)
	testutil.FatalOnErr("WriteString", err, t)
	name := f1.Name()
	err = f1.Close()
	testutil.FatalOnErr("can't close tmpfile", err, t)

	// assign a special user, group and permission for this file
	originUid, originGid, originMod := 1000, 1000, int(0775)
	if err = os.Chmod(name, fs.FileMode(originMod)); err != nil {
		testutil.FatalOnErr("can't change mod of the tmpfile", err, t)
	}

	if err = os.Chown(name, originUid, originGid); err != nil {
		testutil.FatalOnErr("can't change ownership of the tmpfile", err, t)
	}

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewConfClient(conn)
	for _, tc := range []struct {
		name     string
		req      *pb.WriteRequest
		expected string
	}{
		{
			name: "write cluster_file to general",
			expected: `[general]
cluster_file = /tmp/fdb.cluster`,
			req: &pb.WriteRequest{
				Location: &pb.Location{
					File:    name,
					Section: "general",
					Key:     "cluster_file",
				},
				Value: "/tmp/fdb.cluster",
			},
		},
		{
			name: "write to non-existing section",
			expected: `[general]
cluster_file = /tmp/fdb.cluster

[backup.1]
test = badcoffee`,
			req: &pb.WriteRequest{
				Location: &pb.Location{
					File:    name,
					Section: "backup.1",
					Key:     "test",
				},
				Value: "badcoffee",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Write(ctx, tc.req)
			testutil.FatalOnErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			got, err := os.ReadFile(name)
			testutil.FatalOnErr("failed reading config file", err, t)
			sGot := strings.TrimSpace(string(got))
			if sGot != tc.expected {
				t.Errorf("expected: %q, got: %q", tc.expected, sGot)
			}
			// check the new file's permission and ownership
			gotFileInfo, err := os.Stat(tc.req.Location.File)
			testutil.FatalOnErr("can't get file stat info", err, t)
			if gotFileInfo.Mode() != fs.FileMode(originMod) {
				t.Errorf("expected file mode: %q, got: %q", fs.FileMode(originMod), gotFileInfo.Mode())
			}
			if gotFileInfo.Sys().(*syscall.Stat_t).Uid != uint32(originUid) {
				t.Errorf("expected file owner - user id: %q, got: %q", originUid, gotFileInfo.Sys().(*syscall.Stat_t).Uid)
			}
			if gotFileInfo.Sys().(*syscall.Stat_t).Gid != uint32(originGid) {
				t.Errorf("expected file group - group id: %q, got: %q", originGid, gotFileInfo.Sys().(*syscall.Stat_t).Gid)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	temp := t.TempDir()
	f1, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("can't create tmpfile", err, t)
	_, err = f1.WriteString(`[general]
cluster_file = /etc/foundatindb/fdb.cluster

[foo.1]
bar = baz

[foo.2]
bar = baz`)
	testutil.FatalOnErr("WriteString", err, t)
	name := f1.Name()
	err = f1.Close()
	testutil.FatalOnErr("can't close tmpfile", err, t)

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewConfClient(conn)
	for _, tc := range []struct {
		name     string
		req      *pb.DeleteRequest
		expected string
	}{
		{
			name: "delete existing key",
			req: &pb.DeleteRequest{
				Location: &pb.Location{File: name, Section: "foo.2", Key: "bar"},
			},
			expected: `[general]
cluster_file = /etc/foundatindb/fdb.cluster

[foo.1]
bar = baz

[foo.2]`,
		},
		{
			name: "delete empty section",
			req: &pb.DeleteRequest{
				Location: &pb.Location{File: name, Section: "foo.2", Key: ""},
			},
			expected: `[general]
cluster_file = /etc/foundatindb/fdb.cluster

[foo.1]
bar = baz`,
		},
		{
			name: "delete whole section with keys",
			req: &pb.DeleteRequest{
				Location: &pb.Location{File: name, Section: "foo.1", Key: ""},
			},
			expected: `[general]
cluster_file = /etc/foundatindb/fdb.cluster`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Delete(ctx, tc.req)
			testutil.FatalOnErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			got, err := os.ReadFile(name)
			testutil.FatalOnErr("failed reading config file", err, t)
			sGot, sExpected := strings.TrimSpace(string(got)), strings.TrimSpace(tc.expected)
			if sGot != sExpected {
				t.Errorf("expected: %q, got: %q", sExpected, sGot)
			}
		})
	}
}
