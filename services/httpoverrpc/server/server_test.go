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
	"crypto/tls"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
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

	got, err := client.Host(ctx, &httpoverrpc.HostHTTPRequest{
		Request: &httpoverrpc.HTTPRequest{
			Method:     "GET",
			RequestUri: "/",
		},
		Port:     int32(httpPort),
		Protocol: "http",
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

	// test https post request and expect json response
	type Data struct {
		InstanceID int    `json:"instanceId"`
		IPAddress  string `json:"ipAddress"`
	}

	type Response struct {
		Data    Data    `json:"data"`
		Code    *string `json:"code"`
		Message *string `json:"message"`
		Success bool    `json:"success"`
	}
	m = http.NewServeMux()
	m.HandleFunc("/register", func(httpResp http.ResponseWriter, httpReq *http.Request) {
		if httpReq.Method == http.MethodPost {
			httpResp.Header().Set("Content-Type", "application/json")
			response := Response{
				Data: Data{
					InstanceID: 11,
					IPAddress:  "127.0.0.1",
				},
				Code:    nil,
				Message: nil,
				Success: true,
			}
			err = json.NewEncoder(httpResp).Encode(response)
			testutil.FatalOnErr("Failed to ", err, t)
		} else {
			http.Error(httpResp, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	server := httptest.NewTLSServer(m)
	l = server.Listener

	httpClient := server.Client()
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient.Transport = tr

	_, p, err = net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	httpPort, err = strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}

	got, err = client.Host(ctx, &httpoverrpc.HostHTTPRequest{
		Request: &httpoverrpc.HTTPRequest{
			Method:     "POST",
			RequestUri: "/register",
		},
		Port:     int32(httpPort),
		Protocol: "https",
		Hostname: "localhost",
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
	wantBody := `{"data":{"instanceId":11,"ipAddress":"127.0.0.1"},"code":null,"message":null,"success":true}` + "\n"
	contentLengthStr := strconv.Itoa(len(wantBody))
	want = &httpoverrpc.HTTPReply{
		StatusCode: 200,
		Headers: []*httpoverrpc.Header{
			{Key: "Content-Length", Values: []string{contentLengthStr}},
			{Key: "Content-Type", Values: []string{"application/json"}},
			{Key: "Date"},
		},
		Body: []byte(wantBody),
	}
	if !cmp.Equal(got, want, protocmp.Transform()) {
		t.Fatalf("want %v, got %v", want, got)
	}
}
