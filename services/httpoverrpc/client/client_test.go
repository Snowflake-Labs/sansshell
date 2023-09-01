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

package client

import (
	"context"
	"flag"
	"fmt"
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

func TestHTTPTransporter(t *testing.T) {
	ctx := context.Background()

	// Set up web server
	m := http.NewServeMux()
	m.HandleFunc("/helloworld", func(httpResp http.ResponseWriter, httpReq *http.Request) {
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

	// setup http transporter
	transporter := NewHTTPTransporter(conn)

	httpClient := http.Client{
		Transport: transporter,
	}

	addr := l.Addr().String()
	resp, err := httpClient.Get(fmt.Sprintf("http://%s/helloworld", addr))
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

func TestHTTPTransporterBody(t *testing.T) {
	ctx := context.Background()

	// Set up web server
	m := http.NewServeMux()
	m.HandleFunc("/returnbody", func(httpResp http.ResponseWriter, httpReq *http.Request) {
		body := []byte{}
		if httpReq.Body != nil {
			var err error
			body, err = io.ReadAll(httpReq.Body)
			if err != nil {
				_, _ = httpResp.Write([]byte(err.Error()))
			}
		}
		_, _ = httpResp.Write(body)
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

	// setup http transporter
	transporter := NewHTTPTransporter(conn)

	httpClient := http.Client{
		Transport: transporter,
	}

	addr := l.Addr().String()
	reqBody := "hello sansshell"
	resp, err := httpClient.Post(fmt.Sprintf("http://%s/returnbody", addr), "", strings.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	want := reqBody // should receive the sent request body
	if string(respBody) != want {
		t.Errorf("got %q, want %q", respBody, want)
	}
}

func TestHTTPTransporterMissingScheme(t *testing.T) {
	ctx := context.Background()

	// Dial out to sansshell server set up in TestMain
	conn, err := proxy.DialContext(ctx, "", []string{"bufnet"}, grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// setup http transporter
	transporter := NewHTTPTransporter(conn)

	httpClient := http.Client{
		Transport: transporter,
	}

	_, errGet := httpClient.Get("localhost:9090")
	if !strings.Contains(errGet.Error(), errInvalidURLScheme.Error()) {
		t.Fatal("must return error with descriptive message when there's no scheme in the request URL")
	}
}

func TestHTTPTransporterMissingHost(t *testing.T) {
	ctx := context.Background()

	// Dial out to sansshell server set up in TestMain
	conn, err := proxy.DialContext(ctx, "", []string{"bufnet"}, grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// setup http transporter
	transporter := NewHTTPTransporter(conn)

	httpClient := http.Client{
		Transport: transporter,
	}

	_, errGet := httpClient.Get("http://:9090")
	if !strings.Contains(errGet.Error(), errInvalidURLMissingHost.Error()) {
		t.Fatal("must return error with descriptive message when there's no hostname in the request URL")
	}
}

func TestGetPort(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost:9999", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := getPort(req, "http")
	if err != nil {
		t.Fatal(err)
	}
	if result != 9999 {
		t.Fatalf("got wrong port: %d. Expected: %d", result, 9999)
	}
}

func TestGetPortDefaultHTTP(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := getPort(req, "http")
	if err != nil {
		t.Fatal(err)
	}
	if result != defaultHTTPPort {
		t.Fatalf("got wrong port: %d. Expected: %d", result, defaultHTTPPort)
	}
}

func TestGetPortDefaultHTTPS(t *testing.T) {
	req, err := http.NewRequest("GET", "https://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := getPort(req, "https")
	if err != nil {
		t.Fatal(err)
	}
	if result != defaultHTTPSPort {
		t.Fatalf("got wrong port: %d. Expected: %d", result, defaultHTTPSPort)
	}
}
