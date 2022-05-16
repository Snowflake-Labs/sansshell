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
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/sansshell"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestVersion(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	buildVersion := Version
	t.Cleanup(func() { Version = buildVersion })

	client := pb.NewStateClient(conn)
	resp, err := client.Version(ctx, &emptypb.Empty{})
	testutil.FatalOnErr("Version", err, t)
	if got, want := resp.Version, buildVersion; got != want {
		t.Fatalf("didn't get back expected build version. got %s want %s", got, want)
	}
	Version = "versionX"
	resp, err = client.Version(ctx, &emptypb.Empty{})
	testutil.FatalOnErr("Version", err, t)
	if got, want := resp.Version, Version; got != want {
		t.Fatalf("didn't get back expected set version. got %s want %s", got, want)
	}
}
