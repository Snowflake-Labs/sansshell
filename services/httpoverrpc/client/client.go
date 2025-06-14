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
	"github.com/Snowflake-Labs/sansshell/services/util/validator"
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
	listenAddr         string
	allowAnyHost       bool
	protocol           string
	hostname           string
	insecureSkipVerify bool
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
	f.StringVar(&p.protocol, "protocol", "http", "protocol to communicate with specified hostname")
	f.StringVar(&p.hostname, "hostname", "localhost", "ip address or domain name to specify host")
	f.BoolVar(&p.insecureSkipVerify, "insecure-skip-tls-verify", false, "If true, skip TLS cert verification")

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
			Port:      int32(port),
			Protocol:  p.protocol,
			Hostname:  p.hostname,
			Tlsconfig: &pb.TLSConfig{InsecureSkipVerify: p.insecureSkipVerify},
		}
		resp, err := proxy.StreamHost(ctx, req)
		if err != nil {
			sendError(httpResp, http.StatusInternalServerError, err)
			return
		}
		for {
			rs, err := resp.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				sendError(httpResp, http.StatusInternalServerError, err)
			}
			if rs.GetHeader() != nil {
				for _, h := range rs.GetHeader().Headers {
					for _, v := range h.Values {
						httpResp.Header().Add(h.Key, v)
					}
				}
				httpResp.WriteHeader(int(rs.GetHeader().StatusCode))
			}
			if rs.GetBody() != nil {
				if _, err := httpResp.Write(rs.GetBody()); err != nil {
					fmt.Fprintln(os.Stdout, err)
				}
			}
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
	insecureSkipVerify  bool
	dialAddress         string
	stream              bool
}

func (*getCmd) Name() string     { return "get" }
func (*getCmd) Synopsis() string { return "Makes a HTTP(-S) call to a port on a remote host" }
func (*getCmd) Usage() string {
	return `get [-method METHOD] [-header Header...] [-dial-address dialAddress] [-body body] [-protocol Protocol] [-hostname Hostname] [-stream] remoteport request_uri:
  Make a HTTP request to a specified port on the remote host.

  Examples:
  # send get request to https://10.1.23.4:9090/hello
  httpoverrpc get --hostname 10.1.23.4 --protocol https 9090 /hello
  # send get request with url http://example.com:9090/hello, but dial to localhost:9090
  httpoverrpc get --hostname example.com --dialAddress localhost:9090 9090 /hello

  Note:
  1. The prefix / in request_uri is always needed, even there is nothing to put
  2. If we use --hostname to send requests to a specified host instead of the default localhost, and want to use snsshell proxy action
  to proxy requests, don't forget to add --allow-any-host for proxy action
  3. If --stream is set, the response will be streamed back to the client. Useful for large responses. Works only for one target.
`
}

func (g *getCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&g.method, "method", "GET", "Method to use in the HTTP request")
	f.StringVar(&g.protocol, "protocol", "http", "protocol to communicate with specified hostname")
	f.StringVar(&g.hostname, "hostname", "localhost", "ip address or domain name to specify host")
	f.Var(&g.headers, "header", "Header to send in the request, may be specified multiple times.")
	f.StringVar(&g.body, "body", "", "Body to send in request")
	f.BoolVar(&g.showResponseHeaders, "show-response-headers", false, "If true, print response code and headers")
	f.BoolVar(&g.insecureSkipVerify, "insecure-skip-tls-verify", false, "If true, skip TLS cert verification")
	f.StringVar(&g.dialAddress, "dial-address", "", "host:port to dial to. If not provided would dial to original host and port")
	f.BoolVar(&g.stream, "stream", false, "If true, stream the response back to the client")
}

func (g *getCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Please specify exactly two arguments: a port and a query.")
		return subcommands.ExitUsageError
	}
	if len(state.Conn.Targets) > 1 && g.stream {
		fmt.Fprintln(os.Stderr, "Streaming is not supported for multiple targets.")
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

	if g.dialAddress != "" {
		_, _, err := validator.ParseHostAndPort(g.dialAddress)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to parse dial address \"%q\": %v\n", g.dialAddress, err)
			return subcommands.ExitUsageError
		}
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

	var dialConfig *pb.DialConfig = nil
	if g.dialAddress != "" {
		dialConfig = &pb.DialConfig{
			DialAddress: &g.dialAddress,
		}
	}

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
		Tlsconfig: &pb.TLSConfig{
			InsecureSkipVerify: g.insecureSkipVerify,
		},
		Dialconfig: dialConfig,
	}

	proxy := pb.NewHTTPOverRPCClientProxy(state.Conn)
	if g.stream {
		return g.handleStream(ctx, state, &proxy, req)
	}
	return g.handleHost(ctx, state, &proxy, req)
}

func (g *getCmd) handleHost(ctx context.Context, state *util.ExecuteState, proxy *pb.HTTPOverRPCClientProxy, req *pb.HostHTTPRequest) subcommands.ExitStatus {
	resp, err := (*proxy).HostOneMany(ctx, req)
	retCode := subcommands.ExitSuccess
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
			retCode = subcommands.ExitFailure
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
	return retCode
}

func (g *getCmd) handleStream(ctx context.Context, state *util.ExecuteState, proxy *pb.HTTPOverRPCClientProxy, req *pb.HostHTTPRequest) subcommands.ExitStatus {
	resp, err := (*proxy).StreamHostOneMany(ctx, req)
	retCode := subcommands.ExitSuccess
	if err != nil {
		return subcommands.ExitFailure
	}
	for {
		rs, err := resp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(state.Err[0], "Stream failure: %v\n", err)
			retCode = subcommands.ExitFailure
			continue
		}
		for _, r := range rs {
			if r.Error != nil && r.Error != io.EOF {
				fmt.Fprintf(state.Err[r.Index], "%v\n", r.Error)
				return subcommands.ExitFailure
			}
			if g.showResponseHeaders && r.Resp.GetHeader() != nil {
				fmt.Fprintf(state.Out[r.Index], "%v %v\n", r.Resp.GetHeader().StatusCode, http.StatusText(int(r.Resp.GetHeader().StatusCode)))
				for _, h := range r.Resp.GetHeader().Headers {
					for _, v := range h.Values {
						fmt.Fprintf(state.Out[r.Index], "%v: %v\n", h.Key, v)
					}
				}
			}
			if r.Resp.GetBody() != nil {
				fmt.Fprint(state.Out[r.Index], string(r.Resp.GetBody()))
			}
		}
	}
	return retCode
}
