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
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"testing"
)
import "github.com/Snowflake-Labs/sansshell/testing/testutil"

func Test_Shred(t *testing.T) {
	if shredIsUnavailable() {
		t.Skipped()
		return
	}

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)

	t.Cleanup(func() { conn.Close() })
	client := pb.NewLocalFileClient(conn)

	data := "data"
	fileName := createTempFile(data, t)

	_, err = client.Shred(ctx, &pb.ShredRequest{
		Filename: fileName,
		Force:    false,
		Zero:     false,
		Remove:   false,
	})
	testutil.FatalOnErr("shred", err, t)

	if getFileContent(t, fileName) == data {
		t.Fatal("shred did not work")
	}
}

func createTempFile(data string, t *testing.T) string {
	temp := t.TempDir()

	tempFile, err := os.CreateTemp(temp, "testfile.*")
	testutil.FatalOnErr("can't create tmpfile", err, t)

	_, err = tempFile.WriteString(data)
	testutil.FatalOnErr("can't write to tmpfile", err, t)

	err = tempFile.Close()
	testutil.FatalOnErr("closing file", err, t)

	bytes, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("can't read from tmpfile: %v", err)
	}

	if string(bytes) != data {
		t.Fatalf("file contents mismatch: expected %q, got %q", data, string(bytes))
	}

	return tempFile.Name()
}

func getFileContent(t *testing.T, path string) string {
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("can't read file %q: %v", path, err)
	}

	return string(bytes)
}

func shredIsUnavailable() bool {
	path := getShredPath()
	if path == "" {
		return true
	}

	_, err := os.Stat(path)
	return err != nil
}
