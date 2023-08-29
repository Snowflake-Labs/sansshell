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
	"log"
	"net"
	"os"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/Snowflake-Labs/sansshell/services/dns"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
	conn    *grpc.ClientConn
)

type mockResolver struct {
	shouldFail bool
}

func (m mockResolver) LookupIP(ctx context.Context, network, hostname string) ([]net.IP, error) {
	res := []net.IP{}
	if m.shouldFail {
		return res, errors.New("invalid")
	}

	res = append(res, net.IPv4(1, 3, 3, 7))
	return res, nil
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	lfs := &server{}
	lfs.Register(s)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

func TestDnsLookup(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewLookupClient(conn)

	tests := map[string]struct {
		testee  string
		want    []string
		wantErr bool
	}{
		"dns lookup succeeds": {testee: "gotest.com", want: []string{"1.3.3.7"}, wantErr: false},
		"invalid hostname":    {testee: "gotest", wantErr: true},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			origResolver := resolver
			t.Cleanup(func() {
				resolver = origResolver
			})
			resolver = mockResolver{shouldFail: tc.wantErr}.LookupIP

			got, err := client.Lookup(ctx, &pb.LookupRequest{
				Hostname: tc.testee,
			})
			if tc.wantErr {
				testutil.WantErr(name, err, tc.wantErr, t)
				return
			}

			if got, want := got.Result, tc.want; !cmp.Equal(got, want) {
				t.Fatalf("%s: stdout doesn't match. Want %q Got %q", name, want, got)
			}
		})
	}

}
