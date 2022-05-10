package server

import (
	"context"
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"testing"

	pb "github.com/Snowflake-Labs/sansshell/services/fdb"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
	lfs     *server
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	lfs = &server{
		fdbCLIUid: -1,
		fdbCLIGid: -1,
	}
	lfs.Register(s)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

type logDef struct {
	basePath string
	subdir   string
	contents []byte
	perms    fs.FileMode
}

func fixupLogs(logs []captureLogs, def logDef) []captureLogs {
	// Replace each log with our own pattern instead and generate logs so the RPC
	// can process them.
	for i := range logs {
		fp := path.Join(def.basePath, "file")
		logs[i].Path = fp
		if logs[i].IsDir {
			logs[i].Path = path.Join(def.basePath, def.subdir)
			// Create one with a suffix and one without.
			os.WriteFile(path.Join(logs[i].Path, "file"), def.contents, def.perms)
			if logs[i].Suffix != "" {
				os.WriteFile(path.Join(logs[i].Path, "file"+logs[i].Suffix), def.contents, def.perms)
			}
			continue
		}
		os.WriteFile(fp, def.contents, def.perms)
	}
	return logs
}

func TestFDBCLI(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("grpc.DialContext(bufnet)", err, t)
	t.Cleanup(func() { conn.Close() })

	contents := []byte("contents")

	for _, tc := range []struct {
		name    string
		req     *pb.FDBCLIRequest
		resp    *pb.FDBCLIResponse // Set log to filename only. Full path will get filled in below
		wantErr bool
		bin     string
		user    string
		group   string
		subPath bool
		subdir  string
		command []string
		perms   fs.FileMode
	}{
		{
			name:    "missing request",
			req:     &pb.FDBCLIRequest{},
			wantErr: true,
		},
		{
			name: "missing command",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{},
			},
			wantErr: true,
		},
		{
			name: "transaction with no commands",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Transaction{},
			},
			wantErr: true,
		},
		{
			name: "unknown request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Unknown{},
			},
			wantErr: true,
		},
		{
			name: "unknown command",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Unknown{},
					},
				},
			},
			wantErr: true,
		},

		{
			name: "bad binary path",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Defaulttenant{},
					},
				},
			},
			bin:     "cat",
			wantErr: true,
		},
		{
			name: "valid user/group - fails as can't setuid",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Defaulttenant{},
					},
				},
			},
			bin:     testutil.ResolvePath(t, "echo"),
			user:    "nobody",
			group:   "nobody",
			wantErr: true,
		},
		{
			name: "invalid user",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Defaulttenant{},
					},
				},
			},
			bin:     testutil.ResolvePath(t, "echo"),
			user:    "nobody2",
			wantErr: true,
		},
		{
			name: "transaction with bad command",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Transaction{
					Transaction: &pb.FDBCLITransaction{
						Commands: []*pb.FDBCLICommand{
							{
								Command: &pb.FDBCLICommand_Unknown{},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "simple transaction",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Transaction{
					Transaction: &pb.FDBCLITransaction{
						Commands: []*pb.FDBCLICommand{
							{
								Command: &pb.FDBCLICommand_Defaulttenant{},
							},
						},
					},
				},
			},
			resp: &pb.FDBCLIResponse{},
			bin:  testutil.ResolvePath(t, "true"),
			command: []string{
				*fdbCLI,
				"--exec",
				"begin ; defaulttenant ; commit",
			},
		},
		{
			name: "complex transaction",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Transaction{
					Transaction: &pb.FDBCLITransaction{
						Commands: []*pb.FDBCLICommand{
							{
								Command: &pb.FDBCLICommand_Status{},
							},
							{
								Command: &pb.FDBCLICommand_Sleep{
									Sleep: &pb.FDBCLISleep{
										Seconds: 4,
									},
								},
							},
							{
								Command: &pb.FDBCLICommand_Status{},
							},
						},
					},
				},
			},
			resp: &pb.FDBCLIResponse{},
			bin:  testutil.ResolvePath(t, "true"),
			command: []string{
				*fdbCLI,
				"--exec",
				"begin ; status ; sleep 4 ; status ; commit",
			},
		},
		{
			name: "can't read log dir",
			req: &pb.FDBCLIRequest{
				Log: &wrapperspb.BoolValue{
					Value: true,
				},
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Status{},
					},
				},
			},
			bin:     testutil.ResolvePath(t, "true"),
			wantErr: true,
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
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Status{},
					},
				},
			},
			resp: &pb.FDBCLIResponse{
				Logs: []*pb.Log{
					{
						Filename: "file",
						Contents: contents,
					},
				},
			},
			bin: testutil.ResolvePath(t, "true"),
			command: []string{
				*fdbCLI,
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
			perms:   0644,
		},
		{
			name: "advanceversion",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Advanceversion{
							Advanceversion: &pb.FDBCLIAdvanceversion{
								Version: 12345,
							},
						},
					},
				},
			},
			resp: &pb.FDBCLIResponse{},
			bin:  testutil.ResolvePath(t, "true"),
			command: []string{
				*fdbCLI,
				"--exec",
				"advanceversion 12345",
			},
		},
		{
			name: "clear missing key",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Clear{
							Clear: &pb.FDBCLIClear{},
						},
					},
				},
			},
			bin:     testutil.ResolvePath(t, "true"),
			wantErr: true,
		},
		{
			name: "clear",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Clear{
							Clear: &pb.FDBCLIClear{
								Key: "key",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"clear key",
			},
		},
		{
			name: "clearrange missing begin key",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Clearrange{
							Clearrange: &pb.FDBCLIClearrange{},
						},
					},
				},
			},
			bin:     testutil.ResolvePath(t, "true"),
			wantErr: true,
		},
		{
			name: "clearrange missing end key",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Clearrange{
							Clearrange: &pb.FDBCLIClearrange{
								BeginKey: "begin",
							},
						},
					},
				},
			},
			bin:     testutil.ResolvePath(t, "true"),
			wantErr: true,
		},
		{
			name: "clearrange",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Clearrange{
							Clearrange: &pb.FDBCLIClearrange{
								BeginKey: "begin",
								EndKey:   "end",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"clearrange begin end",
			},
		},
		{
			name: "configure",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"configure new triple ssd grv_proxies=3 commit_proxies=4 resolvers=5 logs=6 count=7 perpetual_storage_wiggle=1 perpetual_storage_wiggle_locality=locality storage_migration_type=aggressive tenant_mode=disabled",
			},
		},
		{
			name: "consistencycheck no mode",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Consistencycheck{
							Consistencycheck: &pb.FDBCLIConsistencycheck{},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"consistencycheck",
			},
		},
		{
			name: "consistencycheck with mode off",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Consistencycheck{
							Consistencycheck: &pb.FDBCLIConsistencycheck{
								Mode: &wrapperspb.BoolValue{},
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"consistencycheck off",
			},
		},
		{
			name: "consistencycheck with mode on",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"consistencycheck on",
			},
		},
		{
			name: "coordinators no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Coordinators{
							Coordinators: &pb.FDBCLICoordinators{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "coordinators auto",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"coordinators auto description=descr",
			},
		},
		{
			name: "coordinators addresses",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"coordinators address1 address2 description=descr",
			},
		},
		{
			name: "createtenant no name",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Createtenant{
							Createtenant: &pb.FDBCLICreatetenant{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "createtenant",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Createtenant{
							Createtenant: &pb.FDBCLICreatetenant{
								Name: "name",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"createtenant name",
			},
		},
		{
			name: "defaulttenant",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Defaulttenant{
							Defaulttenant: &pb.FDBCLIDefaulttenant{},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"defaulttenant",
			},
		},
		{
			name: "deletetenant no name",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Deletetenant{
							Deletetenant: &pb.FDBCLIDeletetenant{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "deletetenant",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Deletetenant{
							Deletetenant: &pb.FDBCLIDeletetenant{
								Name: "name",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"deletetenant name",
			},
		},
		{
			name: "exclude",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Exclude{
							Exclude: &pb.FDBCLIExclude{
								Failed: &wrapperspb.BoolValue{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"exclude failed address1 address2",
			},
		},
		{
			name: "fileconfigure no file",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Fileconfigure{
							Fileconfigure: &pb.FDBCLIFileconfigure{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "fileconfigure bad path file",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Fileconfigure{
							Fileconfigure: &pb.FDBCLIFileconfigure{
								File: "/some/path/../../etc/passwd",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "fileconfigure",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"fileconfigure new /some/path",
			},
		},
		{
			name: "force_recovery_with_data_loss no dcid",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_ForceRecoveryWithDataLoss{
							ForceRecoveryWithDataLoss: &pb.FDBCLIForceRecoveryWithDataLoss{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "force_recovery_with_data_loss",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_ForceRecoveryWithDataLoss{
							ForceRecoveryWithDataLoss: &pb.FDBCLIForceRecoveryWithDataLoss{
								Dcid: "dcid",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"force_recovery_with_data_loss dcid",
			},
		},
		{
			name: "get no key",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Get{
							Get: &pb.FDBCLIGet{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "get",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Get{
							Get: &pb.FDBCLIGet{
								Key: "key",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"get key",
			},
		},
		{
			name: "getrange no begin key",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Getrange{
							Getrange: &pb.FDBCLIGetrange{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "getrange",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"getrange key end 123",
			},
		},
		{
			name: "getrangekeys no begin key",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Getrangekeys{
							Getrangekeys: &pb.FDBCLIGetrangekeys{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "getrangekeys",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"getrangekeys key end 123",
			},
		},
		{
			name: "gettenant no name",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Gettenant{
							Gettenant: &pb.FDBCLIGettenant{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "gettenant",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Gettenant{
							Gettenant: &pb.FDBCLIGettenant{
								Name: "tenant",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"gettenant tenant",
			},
		},
		{
			name: "getversion",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Getversion{
							Getversion: &pb.FDBCLIGetversion{},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"getversion",
			},
		},
		{
			name: "help",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"help option1 option2",
			},
		},
		{
			name: "include no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Include{
							Include: &pb.FDBCLIInclude{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "include no addresses",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "include all",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"include failed all",
			},
		},
		{
			name: "include addresses",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"include failed address1 address2",
			},
		},
		{
			name: "kill no address",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"kill",
			},
		},
		{
			name: "kill",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Kill{
							Kill: &pb.FDBCLIKill{
								Addresses: []string{
									"address",
									"address2",
								},
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"kill ; kill address ; kill address2",
			},
		},
		{
			name: "lock",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Lock{
							Lock: &pb.FDBCLILock{},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"lock",
			},
		},
		{
			name: "listtenants",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"listtenants begin end 123",
			},
		},
		{
			name: "maintenance no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Maintenance{
							Maintenance: &pb.FDBCLIMaintenance{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "maintenance status",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Maintenance{
							Maintenance: &pb.FDBCLIMaintenance{
								Request: &pb.FDBCLIMaintenance_Status{},
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"maintenance",
			},
		},
		{
			name: "maintenance on no zoneid",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "maintenance on",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"maintenance on zone 123",
			},
		},
		{
			name: "maintenance off",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"maintenance off",
			},
		},
		{
			name: "option no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Option{
							Option: &pb.FDBCLIOption{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "option blank",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Option{
							Option: &pb.FDBCLIOption{
								Request: &pb.FDBCLIOption_Blank{},
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"option",
			},
		},
		{
			name: "option arg missing state",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "option arg missing option",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "option arg",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"option on option arg",
			},
		},
		{
			name: "profile no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Profile{
							Profile: &pb.FDBCLIProfile{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "profile client no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "profile client get",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"profile client get",
			},
		},
		{
			name: "profile client set no rate",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "profile client set no size",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "profile client set rate and size default",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"profile client set rate default size default",
			},
		},
		{
			name: "profile client set rate and size values",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"profile client set rate 1.2345 size 23456",
			},
		},
		{
			name: "profile list",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"profile list",
			},
		},
		{
			name: "profile flow no processes",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "profile flow",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			resp: &pb.FDBCLIResponse{
				Logs: []*pb.Log{
					{
						Filename: "file",
						Contents: contents,
					},
				},
			},
			command: []string{
				*fdbCLI,
				"--exec",
				"profile flow run 12345 PATH/profile.out process1 process2",
			},
			subPath: true,
			perms:   0644,
		},
		{
			name: "profile heap no process",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "profile heap",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"profile heap process",
			},
		},
		{
			name: "set no key",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Set{
							Set: &pb.FDBCLISet{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "set no value",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Set{
							Set: &pb.FDBCLISet{
								Key: "key",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "set",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Set{
							Set: &pb.FDBCLISet{
								Key:   "key",
								Value: "value",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"set key value",
			},
		},
		{
			name: "setclass no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Setclass{
							Setclass: &pb.FDBCLISetclass{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "setclass list",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"setclass",
			},
		},
		{
			name: "setclass arg no address",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "setclass arg no class",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "setclass arg",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"setclass address class",
			},
		},
		{
			name: "sleep",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Sleep{
							Sleep: &pb.FDBCLISleep{
								Seconds: 123,
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"sleep 123",
			},
		},
		{
			name: "status",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"status json",
			},
		},
		{
			name: "throttle no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Throttle{
							Throttle: &pb.FDBCLIThrottle{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "throttle on no tag",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "throttle on",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"throttle on tag tag1 123 32h high",
			},
		},
		{
			name: "throttle off",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"throttle off all tag tag1 high",
			},
		},
		{
			name: "throttle enable",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"throttle enable auto",
			},
		},
		{
			name: "throttle disble",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"throttle disable auto",
			},
		},
		{
			name: "throttle list",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"throttle list all 123",
			},
		},
		{
			name: "triggerddteaminfolog",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Triggerddteaminfolog{
							Triggerddteaminfolog: &pb.FDBCLITriggerddteaminfolog{},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"triggerddteaminfolog",
			},
		},
		{
			name: "unlock no uid",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Unlock{
							Unlock: &pb.FDBCLIUnlock{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unlock",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Unlock{
							Unlock: &pb.FDBCLIUnlock{
								Uid: "uid",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"unlock uid",
			},
		},
		{
			name: "usetenant no name",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Usetenant{
							Usetenant: &pb.FDBCLIUsetenant{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "usetenant",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Usetenant{
							Usetenant: &pb.FDBCLIUsetenant{
								Name: "name",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"usetenant name",
			},
		},
		{
			name: "writemode no mode",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Writemode{
							Writemode: &pb.FDBCLIWritemode{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "writemode",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Writemode{
							Writemode: &pb.FDBCLIWritemode{
								Mode: "on",
							},
						},
					},
				},
			},
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"writemode on",
			},
		},
		{
			name: "tssq no request",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
						Command: &pb.FDBCLICommand_Tssq{
							Tssq: &pb.FDBCLITssq{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "tssq start no storage uid",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "tssq start",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"tssq start uuid",
			},
		},
		{
			name: "tssq stop no storage uid",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			wantErr: true,
		},
		{
			name: "tssq stop",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"tssq stop uuid",
			},
		},
		{
			name: "tssq list",
			req: &pb.FDBCLIRequest{
				Request: &pb.FDBCLIRequest_Command{
					Command: &pb.FDBCLICommand{
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
			bin:  testutil.ResolvePath(t, "true"),
			resp: &pb.FDBCLIResponse{},
			command: []string{
				*fdbCLI,
				"--exec",
				"tssq list",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			origGen := generateFDBCLIArgs
			origUser := *fdbCLIUser
			origGroup := *fdbCLIGroup
			t.Cleanup(func() {
				generateFDBCLIArgs = origGen
				*fdbCLIUser = origUser
				*fdbCLIGroup = origGroup
				// Reset these so each test starts fresh.
				lfs.fdbCLIUid = -1
				lfs.fdbCLIGid = -1
			})
			*fdbCLIUser = tc.user
			*fdbCLIGroup = tc.group
			var generatedOpts []string
			var logs []captureLogs

			temp := t.TempDir()
			err = os.Mkdir(path.Join(temp, "subdir"), 0755)
			testutil.FatalOnErr("mkdir", err, t)

			generateFDBCLIArgs = func(req *pb.FDBCLIRequest) ([]string, []captureLogs, error) {
				generatedOpts, logs, err = origGen(req)
				logs = fixupLogs(logs, logDef{
					basePath: temp,
					subdir:   tc.subdir,
					contents: contents,
					perms:    tc.perms,
				})
				return []string{tc.bin, strings.Join(generatedOpts, " ")}, logs, err
			}

			client := pb.NewCLIClient(conn)
			resp, err := client.FDBCLI(context.Background(), tc.req)
			t.Log(err)
			testutil.WantErr("call", err, tc.wantErr, t)
			if !tc.wantErr {
				// Fixup our test response as we didn't know the temp dir in the table.
				for _, l := range tc.resp.Logs {
					l.Filename = path.Join(temp, tc.subdir, l.Filename)
				}
				testutil.DiffErr("diff responses", resp, tc.resp, t)
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
