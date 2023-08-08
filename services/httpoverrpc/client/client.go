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
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/httpoverrpc"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "httpoverrpc"

func init() {
	subcommands.Register(&httpCmd{}, subPackage)
}

func (*httpCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	c.Register(&proxyCmd{}, "")
	c.Register(&getCmd{}, "")
	return c
}

type httpCmd struct{}

func (*httpCmd) Name() string { return subPackage }
func (p *httpCmd) Synopsis() string {
	return client.GenerateSynopsis(p.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError)), 2)
}
func (p *httpCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*httpCmd) SetFlags(f *flag.FlagSet) {}

func (p *httpCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := p.GetSubpackage(f)
	return c.Execute(ctx, args...)
}

type proxyCmd struct {
	listenAddr   string
	allowAnyHost bool
}

func (*proxyCmd) Name() string { return "proxy" }
func (*proxyCmd) Synopsis() string {
	return "Starts a web server that proxies to a port on a remote host"
}
func (*proxyCmd) Usage() string {
	return `proxy [-addr ip:port] remoteport:
    Launch a HTTP proxy server that translates HTTP calls into SansShell calls. Any HTTP request to the proxy server will be sent to the sansshell node and translated into a call to the node's localhost on the specified remote port. If -addr is unspecified, it listens on localhost on a random port. Only a single target at a time is supported.
`
}

func (p *proxyCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.listenAddr, "addr", "localhost:0", "Address to listen on, defaults to a random localhost port")
	f.BoolVar(&p.allowAnyHost, "allow-any-host", false, "Serve data regardless of the Host in HTTP requests instead of only allowing localhost and IPs. False by default to prevent DNS rebinding attacks.")
}

// This context detachment is temporary until we use go1.21 and context.WithoutCancel is available.
type noCancel struct {
	ctx context.Context
}

func (c noCancel) Deadline() (time.Time, bool)       { return time.Time{}, false }
func (c noCancel) Done() <-chan struct{}             { return nil }
func (c noCancel) Err() error                        { return nil }
func (c noCancel) Value(key interface{}) interface{} { return c.ctx.Value(key) }

// WithoutCancel returns a context that is never canceled.
func WithoutCancel(ctx context.Context) context.Context {
	return noCancel{ctx: ctx}
}

func sendError(resp http.ResponseWriter, code int, err error) {
	resp.WriteHeader(code)
	if _, err := resp.Write([]byte(err.Error())); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func validatePort(port int) bool {
	return port >= 0 && port <= 65535
}

func (p *proxyCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	// Ignore the parent context timeout because we don't want to time out here.
	ctx = WithoutCancel(ctx)
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Please specify a port to proxy.")
		return subcommands.ExitUsageError
	}
	if len(state.Out) != 1 {
		fmt.Fprintln(os.Stderr, "Proxying can only be done with exactly one target.")
		return subcommands.ExitUsageError
	}
	port, err := strconv.Atoi(f.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Port could not be interpreted as a number.")
		return subcommands.ExitUsageError
	}
	if !validatePort(port) {
		fmt.Fprintln(os.Stderr, "Port could not be outside the range of [0~65535].")
		return subcommands.ExitUsageError
	}

	proxy := pb.NewHTTPOverRPCClientProxy(state.Conn)

	m := http.NewServeMux()
	m.HandleFunc("/", func(httpResp http.ResponseWriter, httpReq *http.Request) {
		host, _, err := net.SplitHostPort(httpReq.Host)
		if err != nil {
			sendError(httpResp, http.StatusBadRequest, err)
			return
		}
		if !p.allowAnyHost && host != "localhost" && net.ParseIP(host) == nil {
			sendError(httpResp, http.StatusBadRequest, errors.New("refusing to serve non-ip non-localhost, set --allow-any-host to allow this call"))
			return
		}

		var reqHeaders []*pb.Header
		for k, v := range httpReq.Header {
			reqHeaders = append(reqHeaders, &pb.Header{Key: k, Values: v})
		}
		body, err := io.ReadAll(httpReq.Body)
		if err != nil {
			sendError(httpResp, http.StatusBadRequest, err)
			return
		}
		req := &pb.HostHTTPRequest{
			Request: &pb.HTTPRequest{
				RequestUri: httpReq.RequestURI,
				Method:     httpReq.Method,
				Headers:    reqHeaders,
				Body:       body,
			},
			Port:     int32(port),
			Protocol: "http",
			Hostname: "localhost",
		}
		resp, err := proxy.Host(ctx, req)
		if err != nil {
			sendError(httpResp, http.StatusInternalServerError, err)
			return
		}
		for _, h := range resp.Headers {
			for _, v := range h.Values {
				httpResp.Header().Add(h.Key, v)
			}
		}
		httpResp.WriteHeader(int(resp.StatusCode))
		if _, err := httpResp.Write(resp.Body); err != nil {
			fmt.Fprintln(os.Stdout, err)
		}
	})
	l, err := net.Listen("tcp4", p.listenAddr)
	if err != nil {
		fmt.Fprintf(state.Err[0], "Unable to listen on %v.\n", p.listenAddr)
		return subcommands.ExitFailure
	}
	fmt.Fprintf(state.Out[0], "Listening on http://%v, ctrl-c to exit...", l.Addr())
	if err := http.Serve(l, m); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return subcommands.ExitUsageError
	}
	return subcommands.ExitSuccess
}

type repeatedString []string

func (i *repeatedString) String() string {
	if i == nil {
		return "[]"
	}
	return fmt.Sprint([]string(*i))
}

func (i *repeatedString) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type getCmd struct {
	method              string
	headers             repeatedString
	body                string
	showResponseHeaders bool
	protocol            string
	hostname            string
}

func (*getCmd) Name() string     { return "get" }
func (*getCmd) Synopsis() string { return "Makes a HTTP call to a port on a remote host" }
func (*getCmd) Usage() string {
	return `get [-method METHOD] [-header Header...] [-body body] [-protocol Protocol] [-hostname Hostname] remoteport request_uri:
    Make a HTTP request to a specified port on the remote host.

	Note: if we set the domain name other than localhost for flag --hostname, and want to use snsshell proxy action to proxy requests
	don't forget to add --allow-any-host for proxy action
`
}

func (g *getCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&g.method, "method", "GET", "Method to use in the HTTP request")
	f.StringVar(&g.protocol, "protocol", "http", "protocol to communicate with specified hostname")
	f.StringVar(&g.hostname, "hostname", "localhost", "ip address or domain name to specify host")
	f.Var(&g.headers, "header", "Header to send in the request, may be specified multiple times.")
	f.StringVar(&g.body, "body", "", "Body to send in request")
	f.BoolVar(&g.showResponseHeaders, "show-response-headers", false, "If true, print response code and headers")
}

func (g *getCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Please specify exactly two arguments: a port and a query.")
		return subcommands.ExitUsageError
	}
	port, err := strconv.Atoi(f.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Port could not be interpreted as a number.")
		return subcommands.ExitUsageError
	}
	if !validatePort(port) {
		fmt.Fprintln(os.Stderr, "Port could not be outside the range of [0~65535].")
		return subcommands.ExitUsageError
	}

	var reqHeaders []*pb.Header
	for _, v := range g.headers {
		split := strings.SplitN(v, ":", 2)
		if len(split) != 2 {
			fmt.Fprintf(os.Stderr, "Unable to parse %q as header, expected \"Key: value\" format", v)
			return subcommands.ExitUsageError
		}
		reqHeaders = append(reqHeaders, &pb.Header{Key: split[0], Values: []string{strings.TrimSpace(split[1])}})
	}

	proxy := pb.NewHTTPOverRPCClientProxy(state.Conn)

	req := &pb.HostHTTPRequest{
		Request: &pb.HTTPRequest{
			RequestUri: f.Arg(1),
			Method:     g.method,
			Headers:    reqHeaders,
			Body:       []byte(g.body),
		},
		Port:     int32(port),
		Protocol: g.protocol,
		Hostname: g.hostname,
	}

	resp, err := proxy.HostOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintf(e, "All targets - could not execute: %v\n", err)
		}
		return subcommands.ExitFailure
	}
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Err[r.Index], "%v\n", r.Error)
			continue
		}
		if g.showResponseHeaders {
			fmt.Fprintf(state.Out[r.Index], "%v %v\n", r.Resp.StatusCode, http.StatusText(int(r.Resp.StatusCode)))
			for _, h := range r.Resp.Headers {
				for _, v := range h.Values {
					fmt.Fprintf(state.Out[r.Index], "%v: %v\n", h.Key, v)

				}
			}
		}
		fmt.Fprintln(state.Out[r.Index], string(r.Resp.Body))
	}
	return subcommands.ExitSuccess
}
