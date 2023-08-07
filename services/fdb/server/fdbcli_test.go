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
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

type logDef struct {
	basePath string
	subdir   string
	contents []byte
	perms    fs.FileMode
}

func fixupLogs(logs []captureLogs, def logDef) ([]captureLogs, error) {
	// Replace each log with our own pattern instead and generate logs so the RPC
	// can process them.
	for i := range logs {
		fp := path.Join(def.basePath, "file")
		logs[i].Path = fp
		if logs[i].IsDir {
			logs[i].Path = path.Join(def.basePath, def.subdir)
			// Create one with a suffix and one without.
			if err := os.WriteFile(path.Join(logs[i].Path, "file"), def.contents, def.perms); err != nil {
				return nil, err
			}
			if logs[i].Suffix != "" {
				if err := os.WriteFile(path.Join(logs[i].Path, "file"+logs[i].Suffix), def.contents, def.perms); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := os.WriteFile(fp, def.contents, def.perms); err != nil {
			return nil, err
		}
	}
	return logs, nil
}

func TestFDBCLI(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	contents := []byte("contents")

	tmpdir := t.TempDir()
	fdbEnvFile := filepath.Join(tmpdir, "fdbenvfile")
	if err := os.WriteFile(fdbEnvFile, []byte("ONE=2\nTHREE=4"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		req        *pb.FDBCLIRequest
		output     *pb.FDBCLIResponseOutput
		respLogs   map[string][]byte // Set log to filename only. Full path will get filled in below
		wantAnyErr bool
		wantLogErr bool
		bin        string
		user       string
		group      string
		subPath    bool
		subdir     string
		command    []string
		perms      fs.FileMode
		envList    []string
		envFile    string
		ignoreOpts bool
	}{
		{
			name:       "missing request",
			req:        &pb.FDBCLIRequest{},
			wantAnyErr: true,
		},
		{
			name: "nil command",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					nil,
				},
			},
			wantAnyErr: true,
		},
		{
			name: "unknown command",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Unknown{},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "bad binary path",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Defaulttenant{},
					},
				},
			},
			bin:        "cat",
			wantAnyErr: true,
		},
		{
			name: "valid user/group - fails as can't setuid",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Defaulttenant{},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "echo"),
			user:       "nobody",
			group:      "nobody",
			wantAnyErr: true,
		},
		{
			name: "invalid user",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Defaulttenant{},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "echo"),
			user:       "nobody2",
			wantAnyErr: true,
		},
		{
			name: "invalid group",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Defaulttenant{},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "echo"),
			group:      "nobody2",
			wantAnyErr: true,
		},
		{
			name: "can't read log file",
			req: &pb.FDBCLIRequest{
				Log: &wrapperspb.BoolValue{
					Value: true,
				},
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Status{},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantLogErr: true,
		},
		{
			name: "can't read log dir",
			req: &pb.FDBCLIRequest{
				Log: &wrapperspb.BoolValue{
					Value: true,
				},
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Status{},
					},
				},
			},
			subdir:     "subdir",
			bin:        testutil.ResolvePath(t, "true"),
			wantLogErr: true,
		},
		{
			name: "stdout check",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Status{},
					},
				},
			},
			output: &pb.FDBCLIResponseOutput{
				Stdout: []byte(fmt.Sprintf("%s --exec status\n", FDBCLI)),
			},
			bin:      testutil.ResolvePath(t, "echo"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"status",
			},
		},
		{
			name: "top level args",
			req: &pb.FDBCLIRequest{
				Config: &wrapperspb.StringValue{
					Value: "config",
				},
				Log: &wrapperspb.BoolValue{
					Value: true,
				},
				TraceFormat: &wrapperspb.StringValue{
					Value: "json",
				},
				TlsCertificateFile: &wrapperspb.StringValue{
					Value: "cert.pem",
				},
				TlsCaFile: &wrapperspb.StringValue{
					Value: "ca.pem",
				},
				TlsKeyFile: &wrapperspb.StringValue{
					Value: "key.pem",
				},
				TlsPassword: &wrapperspb.StringValue{
					Value: "password",
				},
				TlsVerifyPeers: &wrapperspb.StringValue{
					Value: "peers",
				},
				DebugTls: &wrapperspb.BoolValue{
					Value: true,
				},
				Version: &wrapperspb.BoolValue{
					Value: true,
				},
				LogGroup: &wrapperspb.StringValue{
					Value: "group",
				},
				NoStatus: &wrapperspb.BoolValue{
					Value: true,
				},
				Memory: &wrapperspb.StringValue{
					Value: "128M",
				},
				BuildFlags: &wrapperspb.BoolValue{
					Value: true,
				},
				Timeout: &wrapperspb.Int32Value{
					Value: 12345,
				},
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Status{},
					},
				},
			},
			respLogs: map[string][]byte{
				"file": contents,
			},
			bin: testutil.ResolvePath(t, "true"),
			command: []string{
				FDBCLI,
				"--cluster-file",
				"config",
				"--log",
				"--log-dir",
				"PATH",
				"--trace_format",
				"json",
				"--tls_certificate_file",
				"cert.pem",
				"--tls_ca_file",
				"ca.pem",
				"--tls_key_file",
				"key.pem",
				"--tls_password",
				"password",
				"--tls_verify_peers",
				"peers",
				"--debug-tls",
				"--version",
				"--log-group",
				"group",
				"--no-status",
				"--memory",
				"128M",
				"--build-flags",
				"--timeout",
				"12345",
				"--exec",
				"status",
			},
			subPath: true,
			subdir:  "subdir",
			perms:   0755,
		},
		{
			name: "env vars set",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Versionepoch{
							Versionepoch: &pb.FDBCLIVersionepoch{
								Request: &pb.FDBCLIVersionepoch_Info{},
							},
						},
					},
				},
			},
			envList:    []string{"FOO=BAR"},
			envFile:    fdbEnvFile,
			ignoreOpts: true,
			respLogs:   make(map[string][]byte),
			output: &pb.FDBCLIResponseOutput{
				Stdout: []byte("FOO=BAR\nONE=2\nTHREE=4\n"),
			},
			bin: testutil.ResolvePath(t, "env"),
			command: []string{
				FDBCLI,
				"--exec",
				"versionepoch",
			},
		},
		{
			name: "multiple commands",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Begin{
							Begin: &pb.FDBCLIBegin{},
						},
					},
					{
						Command: &pb.FDBCLICommand_Status{
							Status: &pb.FDBCLIStatus{},
						},
					},
					{
						Command: &pb.FDBCLICommand_Commit{
							Commit: &pb.FDBCLICommit{},
						},
					},
				},
			},
			respLogs: make(map[string][]byte),
			bin:      testutil.ResolvePath(t, "true"),
			command: []string{
				FDBCLI,
				"--exec",
				"begin ; status ; commit ;",
			},
		},
		{
			name: "advanceversion",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Advanceversion{
							Advanceversion: &pb.FDBCLIAdvanceversion{
								Version: 12345,
							},
						},
					},
				},
			},
			respLogs: make(map[string][]byte),
			bin:      testutil.ResolvePath(t, "true"),
			command: []string{
				FDBCLI,
				"--exec",
				"advanceversion 12345",
			},
		},
		{
			name: "begin",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Begin{
							Begin: &pb.FDBCLIBegin{},
						},
					},
				},
			},
			respLogs: make(map[string][]byte),
			bin:      testutil.ResolvePath(t, "true"),
			command: []string{
				FDBCLI,
				"--exec",
				"begin",
			},
		},
		{
			name: "blobrange no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Blobrange{
							Blobrange: &pb.FDBCLIBlobrange{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "blobrange start missing keys",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Blobrange{
							Blobrange: &pb.FDBCLIBlobrange{
								Request: &pb.FDBCLIBlobrange_Start{
									Start: &pb.FDBCLIBlobrangeStart{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "blobrange stop missing keys",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Blobrange{
							Blobrange: &pb.FDBCLIBlobrange{
								Request: &pb.FDBCLIBlobrange_Stop{
									Stop: &pb.FDBCLIBlobrangeStop{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "blobrange start",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Blobrange{
							Blobrange: &pb.FDBCLIBlobrange{
								Request: &pb.FDBCLIBlobrange_Start{
									Start: &pb.FDBCLIBlobrangeStart{
										BeginKey: "begin",
										EndKey:   "end",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"blobrange start begin end",
			},
		},
		{
			name: "blobrange stop",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Blobrange{
							Blobrange: &pb.FDBCLIBlobrange{
								Request: &pb.FDBCLIBlobrange_Stop{
									Stop: &pb.FDBCLIBlobrangeStop{
										BeginKey: "begin",
										EndKey:   "end",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"blobrange stop begin end",
			},
		},
		{
			name: "cacherange no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_CacheRange{
							CacheRange: &pb.FDBCLICacheRange{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "cacherange set missing keys",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_CacheRange{
							CacheRange: &pb.FDBCLICacheRange{
								Request: &pb.FDBCLICacheRange_Set{
									Set: &pb.FDBCLICacheRangeSet{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "cacherange clear missing keys",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_CacheRange{
							CacheRange: &pb.FDBCLICacheRange{
								Request: &pb.FDBCLICacheRange_Clear{
									Clear: &pb.FDBCLICacheRangeClear{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "cacherange set",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_CacheRange{
							CacheRange: &pb.FDBCLICacheRange{
								Request: &pb.FDBCLICacheRange_Set{
									Set: &pb.FDBCLICacheRangeSet{
										BeginKey: "begin",
										EndKey:   "end",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"cacherange set begin end",
			},
		},
		{
			name: "cacherange clear",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_CacheRange{
							CacheRange: &pb.FDBCLICacheRange{
								Request: &pb.FDBCLICacheRange_Clear{
									Clear: &pb.FDBCLICacheRangeClear{
										BeginKey: "begin",
										EndKey:   "end",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"cacherange clear begin end",
			},
		},
		{
			name: "changefeed no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "changefeed list",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_List{},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"changefeed list",
			},
		},
		{
			name: "changefeed register blank entries",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Register{
									Register: &pb.FDBCLIChangefeedRegister{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "changefeed register",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Register{
									Register: &pb.FDBCLIChangefeedRegister{
										RangeId: "rangeid",
										Begin:   "begin",
										End:     "end",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"changefeed register rangeid begin end",
			},
		},
		{
			name: "changefeed stop blank entries",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Stop{
									Stop: &pb.FDBCLIChangefeedStop{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "changefeed stop",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Stop{
									Stop: &pb.FDBCLIChangefeedStop{
										RangeId: "rangeid",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"changefeed stop rangeid",
			},
		},
		{
			name: "changefeed destroy blank entries",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Destroy{
									Destroy: &pb.FDBCLIChangefeedDestroy{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "changefeed destroy",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Destroy{
									Destroy: &pb.FDBCLIChangefeedDestroy{
										RangeId: "rangeid",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"changefeed destroy rangeid",
			},
		},
		{
			name: "changefeed stream blank entries",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Stream{
									Stream: &pb.FDBCLIChangefeedStream{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "changefeed stream no range id",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Stream{
									Stream: &pb.FDBCLIChangefeedStream{
										Type: &pb.FDBCLIChangefeedStream_NoVersion{},
									},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "changefeed stream no version",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Stream{
									Stream: &pb.FDBCLIChangefeedStream{
										RangeId: "rangeid",
										Type:    &pb.FDBCLIChangefeedStream_NoVersion{},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"changefeed stream rangeid",
			},
		},
		{
			name: "changefeed stream start version",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Stream{
									Stream: &pb.FDBCLIChangefeedStream{
										RangeId: "rangeid",
										Type: &pb.FDBCLIChangefeedStream_StartVersion{
											StartVersion: &pb.FDBCLIChangefeedStreamStartVersion{
												StartVersion: 12345,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"changefeed stream rangeid 12345",
			},
		},
		{
			name: "changefeed stream start and end version",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Stream{
									Stream: &pb.FDBCLIChangefeedStream{
										RangeId: "rangeid",
										Type: &pb.FDBCLIChangefeedStream_StartEndVersion{
											StartEndVersion: &pb.FDBCLIChangefeedStreamStartEndVersion{
												StartVersion: 12345,
												EndVersion:   23456,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"changefeed stream rangeid 12345 23456",
			},
		},
		{
			name: "changefeed pop blank entries",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Pop{
									Pop: &pb.FDBCLIChangefeedStreamPop{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "changefeed pop",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Changefeed{
							Changefeed: &pb.FDBCLIChangefeed{
								Request: &pb.FDBCLIChangefeed_Pop{
									Pop: &pb.FDBCLIChangefeedStreamPop{
										RangeId: "rangeid",
										Version: 12345,
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"changefeed pop rangeid 12345",
			},
		},
		{
			name: "clear missing key",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Clear{
							Clear: &pb.FDBCLIClear{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "clear",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Clear{
							Clear: &pb.FDBCLIClear{
								Key: "key",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"clear key",
			},
		},
		{
			name: "clearrange missing begin key",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Clearrange{
							Clearrange: &pb.FDBCLIClearrange{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "clearrange missing end key",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Clearrange{
							Clearrange: &pb.FDBCLIClearrange{
								BeginKey: "begin",
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "clearrange",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Clearrange{
							Clearrange: &pb.FDBCLIClearrange{
								BeginKey: "begin",
								EndKey:   "end",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"clearrange begin end",
			},
		},
		{
			name: "commit",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Commit{
							Commit: &pb.FDBCLICommit{},
						},
					},
				},
			},
			respLogs: make(map[string][]byte),
			bin:      testutil.ResolvePath(t, "true"),
			command: []string{
				FDBCLI,
				"--exec",
				"commit",
			},
		},
		{
			name: "configure",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Configure{
							Configure: &pb.FDBCLIConfigure{
								NewOrTss: &wrapperspb.StringValue{
									Value: "new",
								},
								RedundancyMode: &wrapperspb.StringValue{
									Value: "triple",
								},
								StorageEngine: &wrapperspb.StringValue{
									Value: "ssd",
								},
								GrvProxies: &wrapperspb.UInt32Value{
									Value: 3,
								},
								CommitProxies: &wrapperspb.UInt32Value{
									Value: 4,
								},
								Resolvers: &wrapperspb.UInt32Value{
									Value: 5,
								},
								Logs: &wrapperspb.UInt32Value{
									Value: 6,
								},
								Count: &wrapperspb.UInt32Value{
									Value: 7,
								},
								PerpetualStorageWiggle: &wrapperspb.UInt32Value{
									Value: 1,
								},
								PerpetualStorageWiggleLocality: &wrapperspb.StringValue{
									Value: "locality",
								},
								StorageMigrationType: &wrapperspb.StringValue{
									Value: "aggressive",
								},
								TenantMode: &wrapperspb.StringValue{
									Value: "disabled",
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"configure new triple ssd grv_proxies=3 commit_proxies=4 resolvers=5 logs=6 count=7 perpetual_storage_wiggle=1 perpetual_storage_wiggle_locality=locality storage_migration_type=aggressive tenant_mode=disabled",
			},
		},
		{
			name: "consistencycheck no mode",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Consistencycheck{
							Consistencycheck: &pb.FDBCLIConsistencycheck{},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"consistencycheck",
			},
		},
		{
			name: "consistencycheck with mode off",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Consistencycheck{
							Consistencycheck: &pb.FDBCLIConsistencycheck{
								Mode: &wrapperspb.BoolValue{},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"consistencycheck off",
			},
		},
		{
			name: "consistencycheck with mode on",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Consistencycheck{
							Consistencycheck: &pb.FDBCLIConsistencycheck{
								Mode: &wrapperspb.BoolValue{
									Value: true,
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"consistencycheck on",
			},
		},
		{
			name: "coordinators no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Coordinators{
							Coordinators: &pb.FDBCLICoordinators{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "coordinators auto",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Coordinators{
							Coordinators: &pb.FDBCLICoordinators{
								Request: &pb.FDBCLICoordinators_Auto{},
								Description: &wrapperspb.StringValue{
									Value: "descr",
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"coordinators auto description=descr",
			},
		},
		{
			name: "coordinators addresses",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Coordinators{
							Coordinators: &pb.FDBCLICoordinators{
								Request: &pb.FDBCLICoordinators_Addresses{
									Addresses: &pb.FDBCLICoordinatorsAddresses{
										Addresses: []string{
											"address1",
											"address2",
										},
									},
								},
								Description: &wrapperspb.StringValue{
									Value: "descr",
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"coordinators address1 address2 description=descr",
			},
		},
		{
			name: "createtenant no name",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Createtenant{
							Createtenant: &pb.FDBCLICreatetenant{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "createtenant",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Createtenant{
							Createtenant: &pb.FDBCLICreatetenant{
								Name: "name",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"createtenant name",
			},
		},
		{
			name: "datadistribution no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Datadistribution{
							Datadistribution: &pb.FDBCLIDatadistribution{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "datadistribution on",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Datadistribution{
							Datadistribution: &pb.FDBCLIDatadistribution{
								Request: &pb.FDBCLIDatadistribution_On{
									On: &pb.FDBCLIDatadistributionOn{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"datadistribution on",
			},
		},
		{
			name: "datadistribution off",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Datadistribution{
							Datadistribution: &pb.FDBCLIDatadistribution{
								Request: &pb.FDBCLIDatadistribution_Off{
									Off: &pb.FDBCLIDatadistributionOff{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"datadistribution off",
			},
		},
		{
			name: "datadistribution enable no option",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Datadistribution{
							Datadistribution: &pb.FDBCLIDatadistribution{
								Request: &pb.FDBCLIDatadistribution_Enable{
									Enable: &pb.FDBCLIDatadistributionEnable{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "datadistribution enable",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Datadistribution{
							Datadistribution: &pb.FDBCLIDatadistribution{
								Request: &pb.FDBCLIDatadistribution_Enable{
									Enable: &pb.FDBCLIDatadistributionEnable{
										Option: "option",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"datadistribution enable option",
			},
		},
		{
			name: "datadistribution disable no option",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Datadistribution{
							Datadistribution: &pb.FDBCLIDatadistribution{
								Request: &pb.FDBCLIDatadistribution_Disable{
									Disable: &pb.FDBCLIDatadistributionDisable{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "datadistribution disable",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Datadistribution{
							Datadistribution: &pb.FDBCLIDatadistribution{
								Request: &pb.FDBCLIDatadistribution_Disable{
									Disable: &pb.FDBCLIDatadistributionDisable{
										Option: "option",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"datadistribution disable option",
			},
		},
		{
			name: "defaulttenant",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Defaulttenant{
							Defaulttenant: &pb.FDBCLIDefaulttenant{},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"defaulttenant",
			},
		},
		{
			name: "deletetenant no name",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Deletetenant{
							Deletetenant: &pb.FDBCLIDeletetenant{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "deletetenant",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Deletetenant{
							Deletetenant: &pb.FDBCLIDeletetenant{
								Name: "name",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"deletetenant name",
			},
		},
		{
			name: "exclude",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Exclude{
							Exclude: &pb.FDBCLIExclude{
								Failed: &wrapperspb.BoolValue{
									Value: true,
								},
								NoWait: &wrapperspb.BoolValue{
									Value: true,
								},
								Addresses: []string{
									"address1",
									"address2",
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"exclude no_wait failed address1 address2",
			},
		},
		{
			name: "exclude add wait",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Exclude{
							Exclude: &pb.FDBCLIExclude{
								Failed: &wrapperspb.BoolValue{
									Value: true,
								},
								NoWait: &wrapperspb.BoolValue{
									Value: false,
								},
								Addresses: []string{
									"address1",
									"address2",
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"exclude failed address1 address2",
			},
		},
		{
			name: "expensive_data_check no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_ExpensiveDataCheck{
							ExpensiveDataCheck: &pb.FDBCLIExpensiveDataCheck{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "expensive_data_check init",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_ExpensiveDataCheck{
							ExpensiveDataCheck: &pb.FDBCLIExpensiveDataCheck{
								Request: &pb.FDBCLIExpensiveDataCheck_Init{},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"expensive_data_check",
			},
		},
		{
			name: "expensive_data_check list",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_ExpensiveDataCheck{
							ExpensiveDataCheck: &pb.FDBCLIExpensiveDataCheck{
								Request: &pb.FDBCLIExpensiveDataCheck_List{},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"expensive_data_check list",
			},
		},
		{
			name: "expensive_data_check all",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_ExpensiveDataCheck{
							ExpensiveDataCheck: &pb.FDBCLIExpensiveDataCheck{
								Request: &pb.FDBCLIExpensiveDataCheck_All{},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"expensive_data_check all",
			},
		},
		{
			name: "expensive_data_check check no addresses",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_ExpensiveDataCheck{
							ExpensiveDataCheck: &pb.FDBCLIExpensiveDataCheck{
								Request: &pb.FDBCLIExpensiveDataCheck_Check{
									Check: &pb.FDBCLIExpensiveDataCheckCheck{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "expensive_data_check check",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_ExpensiveDataCheck{
							ExpensiveDataCheck: &pb.FDBCLIExpensiveDataCheck{
								Request: &pb.FDBCLIExpensiveDataCheck_Check{
									Check: &pb.FDBCLIExpensiveDataCheckCheck{
										Addresses: []string{"address1", "address2"},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"expensive_data_check address1 address2",
			},
		},
		{
			name: "fileconfigure no file",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Fileconfigure{
							Fileconfigure: &pb.FDBCLIFileconfigure{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "fileconfigure bad path file",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Fileconfigure{
							Fileconfigure: &pb.FDBCLIFileconfigure{
								File: "/some/path/../../etc/passwd",
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "fileconfigure",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Fileconfigure{
							Fileconfigure: &pb.FDBCLIFileconfigure{
								New: &wrapperspb.BoolValue{
									Value: true,
								},
								File: "/some/path",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"fileconfigure new /some/path",
			},
		},
		{
			name: "force_recovery_with_data_loss no dcid",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_ForceRecoveryWithDataLoss{
							ForceRecoveryWithDataLoss: &pb.FDBCLIForceRecoveryWithDataLoss{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "force_recovery_with_data_loss",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_ForceRecoveryWithDataLoss{
							ForceRecoveryWithDataLoss: &pb.FDBCLIForceRecoveryWithDataLoss{
								Dcid: "dcid",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"force_recovery_with_data_loss dcid",
			},
		},
		{
			name: "get no key",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Get{
							Get: &pb.FDBCLIGet{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "get",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Get{
							Get: &pb.FDBCLIGet{
								Key: "key",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"get key",
			},
		},
		{
			name: "getrange no begin key",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Getrange{
							Getrange: &pb.FDBCLIGetrange{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "getrange",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Getrange{
							Getrange: &pb.FDBCLIGetrange{
								BeginKey: "key",
								EndKey: &wrapperspb.StringValue{
									Value: "end",
								},
								Limit: &wrapperspb.UInt32Value{
									Value: 123,
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"getrange key end 123",
			},
		},
		{
			name: "getrangekeys no begin key",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Getrangekeys{
							Getrangekeys: &pb.FDBCLIGetrangekeys{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "getrangekeys",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Getrangekeys{
							Getrangekeys: &pb.FDBCLIGetrangekeys{
								BeginKey: "key",
								EndKey: &wrapperspb.StringValue{
									Value: "end",
								},
								Limit: &wrapperspb.UInt32Value{
									Value: 123,
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"getrangekeys key end 123",
			},
		},
		{
			name: "gettenant no name",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Gettenant{
							Gettenant: &pb.FDBCLIGettenant{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "gettenant",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Gettenant{
							Gettenant: &pb.FDBCLIGettenant{
								Name: "tenant",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"gettenant tenant",
			},
		},
		{
			name: "getversion",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Getversion{
							Getversion: &pb.FDBCLIGetversion{},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"getversion",
			},
		},
		{
			name: "help",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Help{
							Help: &pb.FDBCLIHelp{
								Options: []string{
									"option1",
									"option2",
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"help option1 option2",
			},
		},
		{
			name: "include no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Include{
							Include: &pb.FDBCLIInclude{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "include no addresses",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Include{
							Include: &pb.FDBCLIInclude{
								Request: &pb.FDBCLIInclude_Addresses{
									Addresses: &pb.FDBCLIIncludeAddresses{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "include all",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Include{
							Include: &pb.FDBCLIInclude{
								Failed: &wrapperspb.BoolValue{
									Value: true,
								},
								Request: &pb.FDBCLIInclude_All{},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"include failed all",
			},
		},
		{
			name: "include addresses",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Include{
							Include: &pb.FDBCLIInclude{
								Failed: &wrapperspb.BoolValue{
									Value: true,
								},
								Request: &pb.FDBCLIInclude_Addresses{
									Addresses: &pb.FDBCLIIncludeAddresses{
										Addresses: []string{
											"address1",
											"address2",
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"include failed address1 address2",
			},
		},
		{
			name: "kill no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "kill init",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{
								Request: &pb.FDBCLIKill_Init{
									Init: &pb.FDBCLIKillInit{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"kill",
			},
		},
		{
			name: "kill list",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{
								Request: &pb.FDBCLIKill_List{
									List: &pb.FDBCLIKillList{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"kill list",
			},
		},
		{
			name: "kill all",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{
								Request: &pb.FDBCLIKill_All{
									All: &pb.FDBCLIKillAll{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"kill ; kill all",
			},
		},
		{
			name: "kill targets no addresses",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{
								Request: &pb.FDBCLIKill_Targets{
									Targets: &pb.FDBCLIKillTargets{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "kill targets",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{
								Request: &pb.FDBCLIKill_Targets{
									Targets: &pb.FDBCLIKillTargets{
										Addresses: []string{"address1", "address2"},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"kill ; kill address1 address2",
			},
		},
		{
			name: "kill targets with sleep",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{
								Request: &pb.FDBCLIKill_Targets{
									Targets: &pb.FDBCLIKillTargets{
										Addresses: []string{"address1", "address2"},
									},
								},
								Sleep: &durationpb.Duration{
									Seconds: 5,
									Nanos:   42,
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"kill ; kill address1 address2 ; sleep 5",
			},
		},
		{
			name: "lock",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Lock{
							Lock: &pb.FDBCLILock{},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"lock",
			},
		},
		{
			name: "listtenants",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Listtenants{
							Listtenants: &pb.FDBCLIListtenants{
								Begin: &wrapperspb.StringValue{
									Value: "begin",
								},
								End: &wrapperspb.StringValue{
									Value: "end",
								},
								Limit: &wrapperspb.UInt32Value{
									Value: 123,
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"listtenants begin end 123",
			},
		},
		{
			name: "maintenance no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Maintenance{
							Maintenance: &pb.FDBCLIMaintenance{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "maintenance status",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Maintenance{
							Maintenance: &pb.FDBCLIMaintenance{
								Request: &pb.FDBCLIMaintenance_Status{},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"maintenance",
			},
		},
		{
			name: "maintenance on no zoneid",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Maintenance{
							Maintenance: &pb.FDBCLIMaintenance{
								Request: &pb.FDBCLIMaintenance_On{
									On: &pb.FDBCLIMaintenanceOn{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "maintenance on",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Maintenance{
							Maintenance: &pb.FDBCLIMaintenance{
								Request: &pb.FDBCLIMaintenance_On{
									On: &pb.FDBCLIMaintenanceOn{
										Zoneid:  "zone",
										Seconds: 123,
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"maintenance on zone 123",
			},
		},
		{
			name: "maintenance off",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Maintenance{
							Maintenance: &pb.FDBCLIMaintenance{
								Request: &pb.FDBCLIMaintenance_Off{
									Off: &pb.FDBCLIMaintenanceOff{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"maintenance off",
			},
		},
		{
			name: "option no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Option{
							Option: &pb.FDBCLIOption{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "option blank",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Option{
							Option: &pb.FDBCLIOption{
								Request: &pb.FDBCLIOption_Blank{},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"option",
			},
		},
		{
			name: "option arg missing state",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Option{
							Option: &pb.FDBCLIOption{
								Request: &pb.FDBCLIOption_Arg{
									Arg: &pb.FDBCLIOptionArg{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "option arg missing option",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Option{
							Option: &pb.FDBCLIOption{
								Request: &pb.FDBCLIOption_Arg{
									Arg: &pb.FDBCLIOptionArg{
										State: "on",
									},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "option arg",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Option{
							Option: &pb.FDBCLIOption{
								Request: &pb.FDBCLIOption_Arg{
									Arg: &pb.FDBCLIOptionArg{
										State:  "on",
										Option: "option",
										Arg: &wrapperspb.StringValue{
											Value: "arg",
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"option on option arg",
			},
		},
		{
			name: "profile no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "profile client no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Client{
									Client: &pb.FDBCLIProfileActionClient{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "profile client get",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Client{
									Client: &pb.FDBCLIProfileActionClient{
										Request: &pb.FDBCLIProfileActionClient_Get{
											Get: &pb.FDBCLIProfileActionClientGet{},
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"profile client get",
			},
		},
		{
			name: "profile client set no rate",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Client{
									Client: &pb.FDBCLIProfileActionClient{
										Request: &pb.FDBCLIProfileActionClient_Set{
											Set: &pb.FDBCLIProfileActionClientSet{},
										},
									},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "profile client set no size",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Client{
									Client: &pb.FDBCLIProfileActionClient{
										Request: &pb.FDBCLIProfileActionClient_Set{
											Set: &pb.FDBCLIProfileActionClientSet{
												Rate: &pb.FDBCLIProfileActionClientSet_DefaultRate{
													DefaultRate: &pb.FDBCLIProfileActionClientDefault{},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "profile client set rate and size default",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Client{
									Client: &pb.FDBCLIProfileActionClient{
										Request: &pb.FDBCLIProfileActionClient_Set{
											Set: &pb.FDBCLIProfileActionClientSet{
												Rate: &pb.FDBCLIProfileActionClientSet_DefaultRate{
													DefaultRate: &pb.FDBCLIProfileActionClientDefault{},
												},
												Size: &pb.FDBCLIProfileActionClientSet_DefaultSize{
													DefaultSize: &pb.FDBCLIProfileActionClientDefault{},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"profile client set rate default size default",
			},
		},
		{
			name: "profile client set rate and size values",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Client{
									Client: &pb.FDBCLIProfileActionClient{
										Request: &pb.FDBCLIProfileActionClient_Set{
											Set: &pb.FDBCLIProfileActionClientSet{
												Rate: &pb.FDBCLIProfileActionClientSet_ValueRate{
													ValueRate: 1.2345,
												},
												Size: &pb.FDBCLIProfileActionClientSet_ValueSize{
													ValueSize: 23456,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"profile client set rate 1.2345 size 23456",
			},
		},
		{
			name: "profile list",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_List{
									List: &pb.FDBCLIProfileActionList{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"profile list",
			},
		},
		{
			name: "profile flow no processes",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Flow{
									Flow: &pb.FDBCLIProfileActionFlow{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "profile flow",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Flow{
									Flow: &pb.FDBCLIProfileActionFlow{
										Duration: 12345,
										Processes: []string{
											"process1",
											"process2",
										},
									},
								},
							},
						},
					},
				},
			},
			bin: testutil.ResolvePath(t, "true"),
			respLogs: map[string][]byte{
				"file": contents,
			},
			command: []string{
				FDBCLI,
				"--exec",
				"profile flow run 12345 PATH/profile.out process1 process2",
			},
			subPath: true,
			perms:   0644,
		},
		{
			name: "profile heap no process",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Heap{
									Heap: &pb.FDBCLIProfileActionHeap{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "profile heap",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{
								Request: &pb.FDBCLIProfile_Heap{
									Heap: &pb.FDBCLIProfileActionHeap{
										Process: "process",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"profile heap process",
			},
		},
		{
			name: "set no key",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Set{
							Set: &pb.FDBCLISet{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "set no value",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Set{
							Set: &pb.FDBCLISet{
								Key: "key",
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "set",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Set{
							Set: &pb.FDBCLISet{
								Key:   "key",
								Value: "value",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"set key value",
			},
		},
		{
			name: "setclass no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Setclass{
							Setclass: &pb.FDBCLISetclass{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "setclass list",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Setclass{
							Setclass: &pb.FDBCLISetclass{
								Request: &pb.FDBCLISetclass_List{
									List: &pb.FDBCLISetclassList{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"setclass",
			},
		},
		{
			name: "setclass arg no address",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Setclass{
							Setclass: &pb.FDBCLISetclass{
								Request: &pb.FDBCLISetclass_Arg{
									Arg: &pb.FDBCLISetclassArg{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "setclass arg no class",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Setclass{
							Setclass: &pb.FDBCLISetclass{
								Request: &pb.FDBCLISetclass_Arg{
									Arg: &pb.FDBCLISetclassArg{
										Address: "address",
									},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "setclass arg",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Setclass{
							Setclass: &pb.FDBCLISetclass{
								Request: &pb.FDBCLISetclass_Arg{
									Arg: &pb.FDBCLISetclassArg{
										Address: "address",
										Class:   "class",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"setclass address class",
			},
		},
		{
			name: "sleep",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Sleep{
							Sleep: &pb.FDBCLISleep{
								Seconds: 123,
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"sleep 123",
			},
		},
		{
			name: "snapshot no command",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Snapshot{
							Snapshot: &pb.FDBCLISnapshot{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "snapshot",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Snapshot{
							Snapshot: &pb.FDBCLISnapshot{
								Command: "command",
								Options: []string{"option1", "option2"},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"snapshot command option1 option2",
			},
		},
		{
			name: "status",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Status{
							Status: &pb.FDBCLIStatus{
								Style: &wrapperspb.StringValue{
									Value: "json",
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"status json",
			},
		},
		{
			name: "suspend no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Suspend{
							Suspend: &pb.FDBCLISuspend{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "suspend init",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Suspend{
							Suspend: &pb.FDBCLISuspend{
								Request: &pb.FDBCLISuspend_Init{
									Init: &pb.FDBCLISuspendInit{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"suspend",
			},
		},
		{
			name: "suspend suspend no addresses",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Suspend{
							Suspend: &pb.FDBCLISuspend{
								Request: &pb.FDBCLISuspend_Suspend{
									Suspend: &pb.FDBCLISuspendSuspend{},
								},
							},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "suspend suspend",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Suspend{
							Suspend: &pb.FDBCLISuspend{
								Request: &pb.FDBCLISuspend_Suspend{
									Suspend: &pb.FDBCLISuspendSuspend{
										Seconds:   12.345,
										Addresses: []string{"address1", "address2"},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"suspend 12.345 address1 address2",
			},
		},
		{
			name: "throttle no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Throttle{
							Throttle: &pb.FDBCLIThrottle{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "throttle on no tag",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Throttle{
							Throttle: &pb.FDBCLIThrottle{
								Request: &pb.FDBCLIThrottle_On{
									On: &pb.FDBCLIThrottleActionOn{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "throttle on",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Throttle{
							Throttle: &pb.FDBCLIThrottle{
								Request: &pb.FDBCLIThrottle_On{
									On: &pb.FDBCLIThrottleActionOn{
										Tag: "tag1",
										Rate: &wrapperspb.UInt32Value{
											Value: 123,
										},
										Duration: &wrapperspb.StringValue{
											Value: "32h",
										},
										Priority: &wrapperspb.StringValue{
											Value: "high",
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"throttle on tag tag1 123 32h high",
			},
		},
		{
			name: "throttle off",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Throttle{
							Throttle: &pb.FDBCLIThrottle{
								Request: &pb.FDBCLIThrottle_Off{
									Off: &pb.FDBCLIThrottleActionOff{
										Type: &wrapperspb.StringValue{
											Value: "all",
										},
										Tag: &wrapperspb.StringValue{
											Value: "tag1",
										},
										Priority: &wrapperspb.StringValue{
											Value: "high",
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"throttle off all tag tag1 high",
			},
		},
		{
			name: "throttle enable",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Throttle{
							Throttle: &pb.FDBCLIThrottle{
								Request: &pb.FDBCLIThrottle_Enable{
									Enable: &pb.FDBCLIThrottleActionEnable{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"throttle enable auto",
			},
		},
		{
			name: "throttle disble",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Throttle{
							Throttle: &pb.FDBCLIThrottle{
								Request: &pb.FDBCLIThrottle_Disable{
									Disable: &pb.FDBCLIThrottleActionDisable{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"throttle disable auto",
			},
		},
		{
			name: "throttle list",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Throttle{
							Throttle: &pb.FDBCLIThrottle{
								Request: &pb.FDBCLIThrottle_List{
									List: &pb.FDBCLIThrottleActionList{
										Type: &wrapperspb.StringValue{
											Value: "all",
										},
										Limit: &wrapperspb.UInt32Value{
											Value: 123,
										},
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"throttle list all 123",
			},
		},
		{
			name: "triggerddteaminfolog",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Triggerddteaminfolog{
							Triggerddteaminfolog: &pb.FDBCLITriggerddteaminfolog{},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"triggerddteaminfolog",
			},
		},
		{
			name: "tssq no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Tssq{
							Tssq: &pb.FDBCLITssq{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "tssq start no storage uid",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Tssq{
							Tssq: &pb.FDBCLITssq{
								Request: &pb.FDBCLITssq_Start{
									Start: &pb.FDBCLITssqStart{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "tssq start",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Tssq{
							Tssq: &pb.FDBCLITssq{
								Request: &pb.FDBCLITssq_Start{
									Start: &pb.FDBCLITssqStart{
										StorageUid: "uuid",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"tssq start uuid",
			},
		},
		{
			name: "tssq stop no storage uid",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Tssq{
							Tssq: &pb.FDBCLITssq{
								Request: &pb.FDBCLITssq_Stop{
									Stop: &pb.FDBCLITssqStop{},
								},
							},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "tssq stop",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Tssq{
							Tssq: &pb.FDBCLITssq{
								Request: &pb.FDBCLITssq_Stop{
									Stop: &pb.FDBCLITssqStop{
										StorageUid: "uuid",
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"tssq stop uuid",
			},
		},
		{
			name: "tssq list",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Tssq{
							Tssq: &pb.FDBCLITssq{
								Request: &pb.FDBCLITssq_List{
									List: &pb.FDBCLITssqList{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"tssq list",
			},
		},
		{
			name: "unlock no uid",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Unlock{
							Unlock: &pb.FDBCLIUnlock{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "unlock",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Unlock{
							Unlock: &pb.FDBCLIUnlock{
								Uid: "uid",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"unlock uid",
			},
		},
		{
			name: "usetenant no name",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Usetenant{
							Usetenant: &pb.FDBCLIUsetenant{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "usetenant",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Usetenant{
							Usetenant: &pb.FDBCLIUsetenant{
								Name: "name",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"usetenant name",
			},
		},
		{
			name: "waitconnected",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Waitconnected{
							Waitconnected: &pb.FDBCLIWaitconnected{},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"waitconnected",
			},
		},
		{
			name: "waitopen",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Waitopen{
							Waitopen: &pb.FDBCLIWaitopen{},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"waitopen",
			},
		},
		{
			name: "writemode no mode",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Writemode{
							Writemode: &pb.FDBCLIWritemode{},
						},
					},
				},
			},
			wantAnyErr: true,
		},
		{
			name: "writemode",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Writemode{
							Writemode: &pb.FDBCLIWritemode{
								Mode: "on",
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"writemode on",
			},
		},
		{
			name: "versionepoch no request",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Versionepoch{
							Versionepoch: &pb.FDBCLIVersionepoch{},
						},
					},
				},
			},
			bin:        testutil.ResolvePath(t, "true"),
			wantAnyErr: true,
		},
		{
			name: "versionepoch info",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Versionepoch{
							Versionepoch: &pb.FDBCLIVersionepoch{
								Request: &pb.FDBCLIVersionepoch_Info{
									Info: &pb.FDBCLIVersionepochInfo{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"versionepoch",
			},
		},
		{
			name: "versionepoch get",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Versionepoch{
							Versionepoch: &pb.FDBCLIVersionepoch{
								Request: &pb.FDBCLIVersionepoch_Get{
									Get: &pb.FDBCLIVersionepochGet{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"versionepoch get",
			},
		},
		{
			name: "versionepoch disable",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Versionepoch{
							Versionepoch: &pb.FDBCLIVersionepoch{
								Request: &pb.FDBCLIVersionepoch_Disable{
									Disable: &pb.FDBCLIVersionepochDisable{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"versionepoch disable",
			},
		},
		{
			name: "versionepoch enable",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Versionepoch{
							Versionepoch: &pb.FDBCLIVersionepoch{
								Request: &pb.FDBCLIVersionepoch_Enable{
									Enable: &pb.FDBCLIVersionepochEnable{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"versionepoch enable",
			},
		},
		{
			name: "versionepoch commit",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Versionepoch{
							Versionepoch: &pb.FDBCLIVersionepoch{
								Request: &pb.FDBCLIVersionepoch_Commit{
									Commit: &pb.FDBCLIVersionepochCommit{},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"versionepoch commit",
			},
		},
		{
			name: "versionepoch set",
			req: &pb.FDBCLIRequest{
				Commands: []*pb.FDBCLICommand{
					{
						Command: &pb.FDBCLICommand_Versionepoch{
							Versionepoch: &pb.FDBCLIVersionepoch{
								Request: &pb.FDBCLIVersionepoch_Set{
									Set: &pb.FDBCLIVersionepochSet{
										Epoch: 12345,
									},
								},
							},
						},
					},
				},
			},
			bin:      testutil.ResolvePath(t, "true"),
			respLogs: make(map[string][]byte),
			command: []string{
				FDBCLI,
				"--exec",
				"versionepoch set 12345",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			origGen := generateFDBCLIArgs
			origUser := FDBCLIUser
			origGroup := FDBCLIGroup
			origEnvList := FDBCLIEnvList
			origEnvFile := FDBCLIEnvFile
			t.Cleanup(func() {
				generateFDBCLIArgs = origGen
				FDBCLIUser = origUser
				FDBCLIGroup = origGroup
				FDBCLIEnvList = origEnvList
				FDBCLIEnvFile = origEnvFile
				// Reset these so each test starts fresh.
				lfs.fdbCLIUid = -1
				lfs.fdbCLIGid = -1
			})
			FDBCLIUser = tc.user
			FDBCLIGroup = tc.group
			FDBCLIEnvList = tc.envList
			FDBCLIEnvFile = tc.envFile
			var generatedOpts []string
			var logs []captureLogs

			temp := t.TempDir()
			err = os.Mkdir(path.Join(temp, "subdir"), tc.perms)
			testutil.FatalOnErr("mkdir", err, t)

			generateFDBCLIArgs = func(req *pb.FDBCLIRequest) ([]string, []captureLogs, error) {
				generatedOpts, logs, err = origGen(req)
				var logErr error
				logs, logErr = fixupLogs(logs, logDef{
					basePath: temp,
					subdir:   tc.subdir,
					contents: contents,
					perms:    tc.perms,
				})
				if logErr != nil {
					// Can't error out here since this is executing in the server thread and that won't
					// roll up to our test thread.
					t.Logf("error from fixupLogs: %v", logErr)
					err = logErr
				}
				if tc.ignoreOpts {
					return []string{tc.bin}, logs, err
				}
				return []string{tc.bin, strings.Join(generatedOpts, " ")}, logs, err
			}

			client := pb.NewCLIClient(conn)
			stream, err := client.FDBCLI(context.Background(), tc.req)
			testutil.FatalOnErr("stream setup", err, t)
			respLogs := make(map[string][]byte)
			for {
				resp, err := stream.Recv()
				t.Logf("resp: %+v err: %+v", resp, err)
				if err == io.EOF {
					break
				}
				// This is hard because as a stream we might get an output packet (which has no error)
				// and then an error for a bad log file. We have to handle that.
				if tc.wantAnyErr {
					testutil.FatalOnNoErr("Recv", err, t)
					break
				}
				if tc.output != nil {
					testutil.DiffErr("output", resp.GetOutput(), tc.output, t)
				}
				// We only care about logs from here
				if err == nil && (resp == nil || resp.GetLog() == nil) {
					continue
				}
				testutil.WantErr("Recv", err, tc.wantLogErr, t)
				if tc.wantLogErr {
					break
				}
				log := resp.GetLog()
				respLogs[log.Filename] = append(respLogs[log.Filename], log.Contents...)
			}
			if !tc.wantAnyErr && !tc.wantLogErr {
				// Fixup our test response as we didn't know the temp dir in the table.
				want := make(map[string][]byte)
				for k, v := range tc.respLogs {
					want[path.Join(temp, tc.subdir, path.Base(k))] = v
				}
				testutil.DiffErr("diff responses", respLogs, want, t)
				// The paths are often generated tmp ones so just replace that entry
				// with a placeholder. If this isn't the right entry the diff will fail
				// anyways.
				if tc.subPath {
					for i := range tc.command {
						if tc.command[i] == "PATH" {
							generatedOpts[i] = "PATH"
							break
						}
						idx := strings.Index(tc.command[i], "PATH")
						sl := strings.LastIndex(generatedOpts[i], "/")
						if idx != -1 && sl != -1 {
							after := generatedOpts[i][sl:]
							generatedOpts[i] = generatedOpts[i][:idx] + "PATH" + after
						}
					}
				}
				testutil.DiffErr("diff commands", generatedOpts, tc.command, t)
			}
		})
	}
}
