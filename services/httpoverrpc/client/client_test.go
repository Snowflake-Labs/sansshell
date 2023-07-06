package client

import (
	"context"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/Snowflake-Labs/sansshell/services"
	_ "github.com/Snowflake-Labs/sansshell/services/httpoverrpc/server"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
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

func TestProxy(t *testing.T) {
	ctx := context.Background()

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

	// Dial out to sansshell server set up in TestMain
	conn, err := proxy.DialContext(ctx, "", []string{"bufnet"}, grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Start proxying command
	f := flag.NewFlagSet("proxy", flag.PanicOnError)
	p := &proxyCmd{}
	p.SetFlags(f)
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Parse([]string{port}); err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	go p.Execute(ctx, f, &util.ExecuteState{
		Conn: conn,
		Out:  []io.Writer{writer},
		Err:  []io.Writer{os.Stderr},
	})

	// Find the port to use
	buf := make([]byte, 1024)
	if _, err := reader.Read(buf); err != nil {
		t.Fatal(err)
	}
	msg := strings.Fields(string(buf))
	// Parse out "Listening on http://%v, "
	addr := msg[2][:len(msg[2])-1]

	// Make a call
	resp, err := http.Get(addr)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	want := "hello world"
	if string(body) != want {
		t.Errorf("got %q, want %q", body, want)
	}
}

func TestGet(t *testing.T) {
	ctx := context.Background()

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

	// Dial out to sansshell server set up in TestMain
	conn, err := proxy.DialContext(ctx, "", []string{"bufnet"}, grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Start get command
	f := flag.NewFlagSet("proxy", flag.PanicOnError)
	g := &getCmd{}
	g.SetFlags(f)
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Parse([]string{port, "/"}); err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	go g.Execute(ctx, f, &util.ExecuteState{
		Conn: conn,
		Out:  []io.Writer{writer},
		Err:  []io.Writer{os.Stderr},
	})

	// See if we got the data
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	got := string(buf[:n])
	want := "hello world\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
