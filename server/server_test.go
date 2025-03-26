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
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Snowflake-Labs/sansshell/auth/opa"
	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	hcpb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	_ "github.com/Snowflake-Labs/sansshell/services/healthcheck/server"
	lfpb "github.com/Snowflake-Labs/sansshell/services/localfile"
	_ "github.com/Snowflake-Labs/sansshell/services/localfile/server"
	"github.com/Snowflake-Labs/sansshell/telemetry"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

const (
	policy = `
package sansshell.authz

default allow = false

allow {
    input.type = "LocalFile.ReadActionRequest"
		input.message.file.filename = "/etc/hosts"
}
allow {
    input.type = "LocalFile.ReadActionRequest"
		input.message.file.filename = "/no-such-filename-for-sansshell-unittest"
}
`
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
	conn    *grpc.ClientConn
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func stopSoon(s *grpc.Server) {
	time.Sleep(50 * time.Millisecond)
	s.Stop()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)

	authzPolicy, err := opa.NewAuthzPolicy(context.Background(), policy)
	if err != nil {
		log.Fatalf("Could not build authorizer: %s", err)
		os.Exit(m.Run())
	}
	s, err := BuildServer(
		WithInsecure(),
		WithAuthzPolicy(authzPolicy),
		WithAuthzHook(rpcauth.HostNetHook(lis.Addr())),
	)
	if err != nil {
		log.Fatalf("Could not build server: %s", err)
	}
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

func TestBuildServer(t *testing.T) {
	// Make sure a bad policy fails
	_, err := BuildServer(
		WithLogger(logr.Discard()),
		WithAuthzHook(rpcauth.HostNetHook(lis.Addr())),
	)
	t.Log(err)
	testutil.FatalOnNoErr("empty policy", err, t)
}

func TestServe(t *testing.T) {
	err := Serve("127.0.0.1:0")
	testutil.FatalOnNoErr("empty policy", err, t)

	authzPolicy, err := opa.NewAuthzPolicy(context.Background(), policy)
	if err != nil {
		testutil.FatalOnNoErr("authorizer creation", err, t)
	}

	err = Serve("-", WithAuthzPolicy(authzPolicy))
	testutil.FatalOnNoErr("bad hostport", err, t)
	// Add an 2nd copy of the logging interceptor just to prove adding works.
	err = Serve("127.0.0.1:0",
		WithAuthzPolicy(authzPolicy),
		WithUnaryInterceptor(telemetry.UnaryServerLogInterceptor(logr.Discard())),
		WithStreamInterceptor(telemetry.StreamServerLogInterceptor(logr.Discard())),
		WithOnStartListener(stopSoon),
	)
	testutil.FatalOnErr("Serve 127.0.0.1:0 with extra interceptors", err, t)
}

func TestServeUnix(t *testing.T) {
	socketPath := t.TempDir() + "/test.sock"
	err := ServeUnix(socketPath, nil)
	testutil.FatalOnNoErr("empty policy", err, t)

	authzPolicy, err := opa.NewAuthzPolicy(context.Background(), policy)
	if err != nil {
		testutil.FatalOnNoErr("authorizer creation", err, t)
	}

	// If an existing directory path is given as the socket path, we
	// should fail to start server.
	err = ServeUnix(t.TempDir(), nil, WithAuthzPolicy(authzPolicy))
	testutil.FatalOnNoErr("ServeUnix with directory path", err, t)

	// If the socket already exists, it's OK - it will be removed.
	_, err = net.Listen("unix", socketPath)
	testutil.FatalOnErr("creating socket file in test", err, t)
	fileInfo, err := os.Stat(socketPath)
	testutil.FatalOnErr("getting file info of socket file", err, t)
	if fileInfo.Mode()&os.ModeSocket == 0 {
		t.Fatalf("created socket file is not a socket")
	}
	err = ServeUnix(socketPath,
		nil,
		WithAuthzPolicy(authzPolicy),
		WithOnStartListener(stopSoon))
	testutil.FatalOnErr("ServeUnix with existing socket", err, t)

	// Test the WithSocketConfigHook feature.
	// We set the socket to be world-writable, and check that
	// the hook gets called.
	err = ServeUnix(socketPath,
		func(sp string) error {
			return os.Chmod(sp, os.FileMode(0667))
		},
		WithAuthzPolicy(authzPolicy),
		WithOnStartListener(
			func(srv *grpc.Server) {
				time.Sleep(50 * time.Millisecond)
				stat, err := os.Stat(socketPath)
				testutil.FatalOnErr("getting socket file info", err, t)
				if stat.Mode()&os.ModePerm != 0667 {
					t.Errorf("socket file mode is not 0667 but %o", stat.Mode())
				}
				srv.Stop()
			}))
	testutil.FatalOnErr("ServeUnix with socket config hook", err, t)
}

func TestServerWithUnixCredentials(t *testing.T) {
	socketPath := t.TempDir() + "/test.sock"
	policyTemplateWithUnixCreds := `
package sansshell.authz

default allow = false

allow {
    %s
	input.method = "/HealthCheck.HealthCheck/Ok"
}
`

	// This function produces an on-server-start listener which connects to the
	// server over the Unix socket and calls a gRPC method.
	runHealthCheck := func(t *testing.T, expectedSuccess bool) func(*grpc.Server) {
		return func(s *grpc.Server) {
			defer s.Stop()

			conn, err := grpc.NewClient("passthrough:///unix://"+socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
			testutil.FatalOnErr("Failed to dial bufnet", err, t)
			defer conn.Close()

			client := hcpb.NewHealthCheckClient(conn)
			_, err = client.Ok(context.Background(), &emptypb.Empty{})
			if expectedSuccess {
				testutil.FatalOnErr("Failed to call Ok", err, t)
			} else {
				testutil.FatalOnNoErr("Ok should have failed", err, t)
			}
		}
	}

	// In the test environment, the process connecting over the Unix socket
	// will be the same process as the one running the server. We can rely on
	// that to get the expected values for the "peer's" Unix credentials.
	currentUser, err := user.Current()
	testutil.FatalOnErr("Failed to get current user", err, t)
	currentUid, err := strconv.Atoi(currentUser.Uid)
	testutil.FatalOnErr("Failed to convert current user UID to int", err, t)

	// We will check that both the primary and supplementary groups can be
	// used in RPC authz.
	groupIdStrings, err := currentUser.GroupIds()
	testutil.FatalOnErr("Failed to get group IDs of current user", err, t)
	groupNameStrings := []string{}
	for _, groupIdString := range groupIdStrings {
		groupInfo, err := user.LookupGroupId(groupIdString)
		testutil.FatalOnErr("Failed to get group info", err, t)
		groupNameStrings = append(groupNameStrings, groupInfo.Name)
	}

	for _, tc := range []struct {
		name            string
		policyFragment  string
		expectedSuccess bool
	}{
		{
			name:            "UID match",
			policyFragment:  fmt.Sprintf("input.peer.unix.uid ==  %d", currentUid),
			expectedSuccess: true,
		},
		{
			name:            "UID mismatch",
			policyFragment:  fmt.Sprintf("input.peer.unix.uid ==  %d", currentUid+1),
			expectedSuccess: false,
		},
		{
			name:            "username match",
			policyFragment:  fmt.Sprintf("input.peer.unix.username ==  \"%s\"", currentUser.Username),
			expectedSuccess: true,
		},
		{
			name:            "username mismatch",
			policyFragment:  fmt.Sprintf("input.peer.unix.username ==  \"%s\"", currentUser.Username+"x"),
			expectedSuccess: false,
		},
		{
			name:            "primary GID match",
			policyFragment:  fmt.Sprintf("input.peer.unix.gids[_] == %s", groupIdStrings[0]),
			expectedSuccess: true,
		},
		{
			name:            "supplementary GID match",
			policyFragment:  fmt.Sprintf("input.peer.unix.gids[_] == %s", groupIdStrings[len(groupIdStrings)-1]),
			expectedSuccess: true,
		},
		{
			name:            "primary group name match",
			policyFragment:  fmt.Sprintf("input.peer.unix.groupnames[_] == \"%s\"", groupNameStrings[0]),
			expectedSuccess: true,
		},
		{
			name:            "supplementary group name match",
			policyFragment:  fmt.Sprintf("input.peer.unix.groupnames[_] == \"%s\"", groupNameStrings[len(groupNameStrings)-1]),
			expectedSuccess: true,
		},
		{
			name:            "group name mismatch",
			policyFragment:  fmt.Sprintf("input.peer.unix.groupnames[_] == \"%s\"", groupNameStrings[0]+"x"),
			expectedSuccess: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := fmt.Sprintf(policyTemplateWithUnixCreds, tc.policyFragment)
			ctx := context.Background()
			authzPolicy, err := opa.NewAuthzPolicy(ctx, policy)
			testutil.FatalOnErr("Policy creation", err, t)

			err = ServeUnix(socketPath,
				nil,
				WithAuthzPolicy(authzPolicy),
				WithCredentials(NewUnixPeerTransportCredentials()),
				WithOnStartListener(runHealthCheck(t, tc.expectedSuccess)))
			testutil.FatalOnErr("ServeUnix with Unix creds policy", err, t)
		})
	}
}

func TestRead(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	for _, tc := range []struct {
		filename string
		err      string
	}{
		{
			filename: "/etc/hosts",
			err:      "",
		},
		{
			filename: "/no-such-filename-for-sansshell-unittest",
			err:      "no such file or directory",
		},
		{
			filename: "/permission-denied-filename-for-sansshell-unittest",
			err:      "PermissionDenied",
		},
	} {
		tc := tc
		t.Run(tc.filename, func(t *testing.T) {
			client := lfpb.NewLocalFileClient(conn)
			stream, err := client.Read(ctx, &lfpb.ReadActionRequest{
				Request: &lfpb.ReadActionRequest_File{
					File: &lfpb.ReadRequest{
						Filename: tc.filename,
					},
				},
			})
			if err != nil {
				// At this point it only returns if we can't connect. Actual errors
				// happen below on the first stream Recv() call.
				if tc.err == "" {
					t.Fatalf("Start of Read failed: %v", err)
				}
			}
			buf := &bytes.Buffer{}
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Logf("Got error: %v", err)
					if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
						t.Errorf("unexpected error; want: %s, got: %s", tc.err, err)
					}
					// If this was an expected error we're done.
					return
				}

				contents := resp.GetContents()
				n, err := buf.Write(contents)
				if got, want := n, len(contents); got != want {
					t.Fatalf("Can't write into buffer at correct length. Got %d want %d", got, want)
				}
				testutil.FatalOnErr("Can't write into buffer", err, t)
			}

			contents, err := os.ReadFile(tc.filename)
			testutil.FatalOnErr("reading test data", err, t)
			if got, want := buf.Bytes(), contents; !bytes.Equal(got, want) {
				t.Fatalf("contents do not match. Got:\n%s\n\nWant:\n%s", got, want)
			}
		})
	}
}
