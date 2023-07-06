package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/Snowflake-Labs/sansshell/services/httpoverrpc"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"
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

func TestServer(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := httpoverrpc.NewHTTPOverRPCClient(conn)

	// Set up web server
	m := http.NewServeMux()
	m.HandleFunc("/", func(httpResp http.ResponseWriter, httpReq *http.Request) {
		_, _ = httpResp.Write([]byte("hello world"))
	})
	l, err := net.Listen("tcp4", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = http.Serve(l, m) }()

	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	httpPort, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.Localhost(ctx, &httpoverrpc.LocalhostHTTPRequest{
		Request: &httpoverrpc.HTTPRequest{
			Method:     "GET",
			RequestUri: "/",
		},
		Port: int32(httpPort),
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Slice(got.Headers, func(i, j int) bool {
		return got.Headers[i].Key < got.Headers[j].Key
	})
	for _, h := range got.Headers {
		if h.Key == "Date" {
			// Clear out the date header because it varies based on time.
			h.Values = nil
		}
	}

	want := &httpoverrpc.HTTPReply{
		StatusCode: 200,
		Headers: []*httpoverrpc.Header{
			{Key: "Content-Length", Values: []string{"11"}},
			{Key: "Content-Type", Values: []string{"text/plain; charset=utf-8"}},
			{Key: "Date"},
		},
		Body: []byte("hello world"),
	}
	if !cmp.Equal(got, want, protocmp.Transform()) {
		t.Fatalf("want %v, got %v", want, got)
	}
}
