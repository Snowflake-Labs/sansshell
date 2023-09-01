/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

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

// Package client provides the client interface for 'httpoverrpc'
package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/httpoverrpc"
)

const (
	defaultHTTPPort  = 80
	defaultHTTPSPort = 443
)

var (
	errInvalidURLScheme      = fmt.Errorf("invalid URL scheme. Use either 'http' or 'https'")
	errInvalidURLMissingHost = fmt.Errorf("no host in the request URL")
)

type HTTPTransporter struct {
	conn *proxy.Conn
}

func NewHTTPTransporter(conn *proxy.Conn) *HTTPTransporter {
	return &HTTPTransporter{
		conn,
	}
}

func httpHeaderToPbHeader(h *http.Header) []*pb.Header {
	result := []*pb.Header{}
	for k, v := range *h {
		result = append(result, &pb.Header{
			Key:    k,
			Values: v,
		})
	}

	return result
}

func pbHeaderToHTTPHeader(header []*pb.Header) http.Header {
	result := http.Header{}
	for _, h := range header {
		result[h.Key] = h.Values
	}

	return result
}

func pbReplytoHTTPResponse(rep *pb.HTTPReply) *http.Response {
	reader := bytes.NewReader(rep.Body)
	body := io.NopCloser(reader)
	header := pbHeaderToHTTPHeader(rep.Headers)
	result := &http.Response{
		Body:       body,
		StatusCode: int(rep.StatusCode),
		Header:     header,
	}

	return result
}

// getPort retrieves the port number from the request URL.
// If the URL doesn't contain a port number, it returns the
// default port associated with the HTTP protocol.
func getPort(req *http.Request, protocol string) (int32, error) {
	var ret int32
	if req.URL.Port() != "" {
		port, err := strconv.Atoi(req.URL.Port())
		if err != nil {
			return 0, err
		}
		ret = int32(port)
	} else {
		// No port in URL, add default port
		if protocol == "http" {
			ret = defaultHTTPPort
		} else {
			ret = defaultHTTPSPort
		}
	}

	return ret, nil
}

func (c *HTTPTransporter) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, errInvalidURLScheme
	}

	if req.URL.Hostname() == "" {
		return nil, errInvalidURLMissingHost
	}

	proxy := pb.NewHTTPOverRPCClientProxy(c.conn)
	body := []byte{}
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}
	reqPb := &pb.HostHTTPRequest{
		Request: &pb.HTTPRequest{
			RequestUri: req.URL.Path,
			Method:     req.Method,
			Headers:    httpHeaderToPbHeader(&req.Header),
			Body:       body,
		},
		Protocol: req.URL.Scheme,
		Hostname: req.URL.Hostname(),
	}

	port, errPort := getPort(req, reqPb.Protocol)
	if errPort != nil {
		return nil, fmt.Errorf("error getting port: %v", errPort)
	}
	reqPb.Port = port

	respChan, err := proxy.HostOneMany(req.Context(), reqPb)
	if err != nil {
		return nil, err
	}
	resp := <-respChan
	if resp.Error != nil {
		return nil, fmt.Errorf("httpOverRPC failed: %v", resp.Error)
	}
	result := pbReplytoHTTPResponse(resp.Resp)
	return result, nil
}
