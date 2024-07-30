/* Copyright (c) 2024 Snowflake Inc. All rights reserved.

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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"testing"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/Snowflake-Labs/sansshell/services"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
	"github.com/gowebpki/jcs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	// Used by our tests
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/server"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	for _, svc := range services.ListServices() {
		svc.Register(s)
	}
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

// deterministicJSON reformats an input of 0 or more json objects
// to be deterministic. Normal protojson unmarshalling is explicitly
// nondeterministic.
func deterministicJSON(in []byte) ([]byte, error) {
	var out []byte
	dec := json.NewDecoder(bytes.NewReader(in))
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		got, err := jcs.Transform([]byte(raw))
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

func TestRPCs(t *testing.T) {
	ctx := context.Background()

	f, err := os.CreateTemp("", "sansshell-test")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile := f.Name()
	defer os.Remove(tmpFile)
	fmt.Fprintln(f, "hi")
	f.Close()

	// Dial out to sansshell server set up in TestMain
	conn, err := proxy.DialContext(ctx, "", []string{"bufnet"}, grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	for _, tc := range []struct {
		desc           string
		args           []string
		want           string
		wantExitStatus subcommands.ExitStatus
	}{
		{
			desc: "unary RPC",
			args: []string{"/HealthCheck.HealthCheck/Ok", "{}"},
			want: "{}",
		},
		{
			desc: "with metadata",
			args: []string{"-metadata=foo=bar", "-metadata=baz=qux", "/HealthCheck.HealthCheck/Ok", "{}"},
			want: "{}",
		},
		{
			desc: "server-streaming RPC",
			args: []string{
				"/LocalFile.LocalFile/Read",
				`{"file":{"filename": "` + tmpFile + `"}}`,
			},
			want: `{"contents":"aGkK"}`,
		},
		{
			desc: "bidirectional-streaming RPC",
			args: []string{
				"/LocalFile.LocalFile/Sum",
				`{"filename": "` + tmpFile + `"}`,
			},
			want: `{"filename":"` + tmpFile + `","sum":"98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4","sum_type":"SUM_TYPE_SHA256"}`,
		},
		{
			desc: "bidirectional-streaming RPC with multiple messages",
			args: []string{
				"/LocalFile.LocalFile/Sum",
				`{"filename": "` + tmpFile + `"}
				  {"filename": "` + tmpFile + `"}`,
			},
			want: `{"filename":"` + tmpFile + `","sum":"98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4","sum_type":"SUM_TYPE_SHA256"}{"filename":"` + tmpFile + `","sum":"98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4","sum_type":"SUM_TYPE_SHA256"}`,
		},
		{
			desc:           "nonexistent message",
			args:           []string{"/Wrong", "{}"},
			wantExitStatus: subcommands.ExitUsageError,
		},
		{
			desc:           "too few args",
			args:           []string{},
			wantExitStatus: subcommands.ExitUsageError,
		},
		{
			desc:           "too many args",
			args:           []string{"/HealthCheck.HealthCheck/Ok", "{}", "{}"},
			wantExitStatus: subcommands.ExitUsageError,
		},
		{
			desc:           "bad metadata",
			args:           []string{"-metadata=foo", "/HealthCheck.HealthCheck/Ok", "{}"},
			wantExitStatus: subcommands.ExitUsageError,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var output bytes.Buffer
			f := flag.NewFlagSet("call", flag.PanicOnError)
			p := &callCmd{}
			p.SetFlags(f)
			if err := f.Parse(tc.args); err != nil {
				t.Fatalf("unable to set flags: %v", err)
			}
			res := p.Execute(ctx, f, &util.ExecuteState{
				Conn: conn,
				Out:  []io.Writer{&output},
				Err:  []io.Writer{os.Stderr},
			})

			if res != tc.wantExitStatus {
				t.Errorf("unexpected subcommand status %v", res)
			}

			got, err := deterministicJSON(output.Bytes())
			if err != nil {
				t.Error(err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}
