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
