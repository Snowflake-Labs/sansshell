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

// Package client provides functionality so that other uses of sansshell can provide their
// own main.go without having to cargo-cult everything across for common use cases.
// i.e. adding additional modules that are locally defined.
package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc/credentials"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/Snowflake-Labs/sansshell/services/whoami/client"

	cmdUtil "github.com/Snowflake-Labs/sansshell/cmd/util"
	"github.com/Snowflake-Labs/sansshell/services/mpa/mpahooks"
	"github.com/Snowflake-Labs/sansshell/services/util"
	writerUtils "github.com/Snowflake-Labs/sansshell/services/util/writer"
)

func init() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
}

// RunState encapsulates all of the variable state needed
// to run a sansssh command.
type RunState struct {
	// Proxy is an optional proxy server to route requests.
	Proxy string
	// Targets is a list of remote targets to use when a proxy
	// is in use. For non proxy must be 1 entry.
	Targets []string
	// Outputs must map 1:1 with Targets indicating where to emit
	// output from commands. If the list is empty or a single entry
	// set to - then stdout/stderr will be used for all outputs.
	Outputs []string
	// OutputsDir defines a directory to place outputs instead of
	// specifying then in Outputs. The files will be names 0.output,
	// 1.output and .error respectively for each target.
	OutputsDir string
	// CredSource is a registered credential source with the mtls package.
	CredSource string
	// IdleTimeout is the time duration to wait before closing an idle connection.
	// If no messages are sent/received within this timeframe, connection will be terminated.
	IdleTimeout time.Duration
	// ClientAuthzPolicy is an optional authz policy for determining outbound decisions.
	ClientAuthzPolicy rpcauth.AuthzPolicy
	// PrefixOutput if true will prefix every line of output with '<index>-<target>: '
	PrefixOutput bool
	// BatchSize if non-zero will do the requested operation to the targets but in
	// N calls to the proxy where N is the target list size divided by BatchSize.
	BatchSize int
	// If true, add an interceptor that performs the multi-party auth flow
	EnableMPA bool
	// If true, configure MPA interceptor to request approval method-wide.
	EnableMethodWideMPA bool
	// If true, the command is authz dry run and real action should not be executed
	AuthzDryRun bool

	// Interspectors for unary calls to the connection to the proxy
	ClientUnaryInterceptors []proxy.UnaryInterceptor
	// Interspectors for streaming calls to the connection to the proxy
	ClientStreamInterceptors []proxy.StreamInterceptor

	credentials.PerRPCCredentials
}

const (
	defaultOutput = "-"
)

// UnaryClientTimeoutInterceptor returns a grpc.UnaryClientInterceptor that adds a deadline to every request
func UnaryClientTimeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientTimeoutInterceptor returns a grpc.UnaryClientInterceptor that adds a deadline to every client SendMsg and RecvMsg
func StreamClientTimeoutInterceptor(timeout time.Duration) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, err
		}
		t := &clientStreamWithTimeout{ClientStream: clientStream, timeout: timeout}
		return t, nil
	}
}

type clientStreamWithTimeout struct {
	grpc.ClientStream
	timeout time.Duration
}

func (t *clientStreamWithTimeout) SendMsg(m interface{}) error {
	ctx := t.Context()
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- t.ClientStream.SendMsg(m)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		err := ctx.Err()
		if err == context.DeadlineExceeded {
			err = fmt.Errorf("connection closed due to no activity after %s: %v", t.timeout, err)
		}
		select {
		// if there's any err in errCh, append it to err
		case errSend := <-errCh:
			return fmt.Errorf("%s - %s", err, errSend)
		// otherwise just return err
		default:
			return err
		}
	}
}

func (t *clientStreamWithTimeout) RecvMsg(m interface{}) error {
	ctx, cancel := context.WithTimeout(t.Context(), t.timeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- t.ClientStream.RecvMsg(m)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():

		select {
		// if there's any err in errCh, just return it
		case errRecv := <-errCh:
			return errRecv
		// otherwise just return err
		default:
			err := ctx.Err()
			if err == context.DeadlineExceeded {
				return fmt.Errorf("connection closed due to no activity after %s: %v", t.timeout, err)
			}
			return err
		}
	}
}

// Run takes the given context and RunState and executes the command passed in after parsing with flags.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, rs RunState) {
	// If we're running internal commands we don't need anything else.
	for i, f := range flag.Args() {
		// Omits unneeded part of the setup for internal commands, but only when they're passed as arguments in the beginning of the command (up to 2nd position).
		// This prevents us from parsing e.g. user input `sanssh service enable help` as `sanssh help` and running `enable` command without
		// the proper setup (e.g. a valid connection to sansshell-server), resulting in an error.
		if i > 1 {
			break
		}
		switch f {
		case "help", "flags", "commands":
			os.Exit(int(subcommands.Execute(ctx, &util.ExecuteState{})))
		case "whoami":
			cmd := &client.WhoamiCommand{}
			ctx := context.Background()
			fs := flag.NewFlagSet("whoami", flag.ExitOnError)
			os.Exit(int(cmd.Execute(ctx, fs, client.WhoamiParams{CredSource: rs.CredSource, ProxyHost: rs.Proxy})))
		}
	}
	if len(flag.Args()) <= 1 {
		// If there's no flags or only one flag, whoever's running this is probably still learning how
		// to invoke the tool and not trying to run the command.
		os.Exit(int(subcommands.Execute(ctx, &util.ExecuteState{})))
	}

	// Bunch of flag sanity checking
	if len(rs.Targets) == 0 && rs.Proxy == "" {
		fmt.Fprintln(os.Stderr, "Must set a target or a proxy")
		os.Exit(1)
	}
	if len(rs.Targets) > 1 && rs.Proxy == "" {
		fmt.Fprintln(os.Stderr, "Can't set targets to multiple entries without a proxy")
		os.Exit(1)
	}

	// Process combinations of outputs/output-dir that are valid and in the end
	// make sure outputsFlag has the correct relevant entries.
	var dir string
	if rs.OutputsDir != "" {
		if len(rs.Outputs) > 0 {
			fmt.Fprintln(os.Stderr, "Can't set outputs and output-dir at the same time.")
			os.Exit(1)
		}
		o, err := os.Stat(rs.OutputsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't open %s: %v\n", rs.OutputsDir, err)
			os.Exit(1)
		}
		if !o.Mode().IsDir() {
			fmt.Fprintf(os.Stderr, "%s: is not a directory\n", rs.OutputsDir)
			os.Exit(1)
		}
		// Generate outputs from output-dir and the target count. The final filename is $output-dir/NUM-TARGET
		// as often one wants to correlate specific output back to a given target and there's no guarentee the
		// output will have this information in it. So instead supply it as metadata via the filename.
		for i, t := range rs.Targets {
			targetName := cmdUtil.StripTimeout(t)
			rs.Outputs = append(rs.Outputs, filepath.Join(rs.OutputsDir, fmt.Sprintf("%d-%s", i, targetName)))
		}
		dir = rs.OutputsDir
	} else {
		// No outputs or outputs-dir so we default to -. It'll process below.
		if len(rs.Outputs) == 0 {
			rs.Outputs = append(rs.Outputs, defaultOutput)
		}

		if len(rs.Outputs) != len(rs.Targets) {
			// Special case. We allow a single - and everything goes to stdout/stderr.
			if !(len(rs.Outputs) == 1 && rs.Outputs[0] == defaultOutput) {
				fmt.Fprintln(os.Stderr, "outputs and targets must contain the same number of entries")
				os.Exit(1)
			}
			if len(rs.Outputs) > 1 && rs.Outputs[0] == defaultOutput {
				fmt.Fprintf(os.Stderr, "outputs using %q can only have one entry\n", defaultOutput)
				os.Exit(1)
			}
			// Now if we have - passed we'll autofill it into the remaining slots for processing below.
			if rs.Outputs[0] == defaultOutput {
				for i := 1; i < len(rs.Targets); i++ {
					rs.Outputs = append(rs.Outputs, defaultOutput)
				}
			}
		}
		var err error
		dir, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't determine current directory: %v", err)
			os.Exit(1)
		}
	}
	creds, err := mtls.LoadClientCredentials(ctx, rs.CredSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load creds from %s - %v\n", rs.CredSource, err)
		os.Exit(1)
	}

	var clientAuthz rpcauth.RPCAuthorizer
	if rs.ClientAuthzPolicy != nil {
		clientAuthz = rpcauth.NewRPCAuthorizer(rs.ClientAuthzPolicy)
	}

	// We may need an option for doing client OPA checks.
	ops := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		// Use 16MB instead of the default 4MB to allow larger responses
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16 * 1024 * 1024)),
	}

	if rs.PerRPCCredentials != nil {
		ops = append(ops, grpc.WithPerRPCCredentials(rs.PerRPCCredentials))
	}

	streamInterceptors := []grpc.StreamClientInterceptor{}
	unaryInterceptors := []grpc.UnaryClientInterceptor{}
	if clientAuthz != nil {
		streamInterceptors = append(streamInterceptors, clientAuthz.AuthorizeClientStream)
		unaryInterceptors = append(unaryInterceptors, clientAuthz.AuthorizeClient)
	}
	if rs.EnableMPA {
		unaryInterceptors = append(unaryInterceptors, mpahooks.UnaryClientIntercepter(rs.EnableMethodWideMPA))
		streamInterceptors = append(streamInterceptors, mpahooks.StreamClientIntercepter(rs.EnableMethodWideMPA))
	}
	// timeout interceptor should be the last item in ops so that it's executed first.
	streamInterceptors = append(streamInterceptors, StreamClientTimeoutInterceptor(rs.IdleTimeout))
	unaryInterceptors = append(unaryInterceptors, UnaryClientTimeoutInterceptor(rs.IdleTimeout))

	ops = append(ops,
		grpc.WithChainStreamInterceptor(streamInterceptors...),
		grpc.WithChainUnaryInterceptor(unaryInterceptors...),
	)

	state := &util.ExecuteState{
		Dir: dir,
	}

	makeWriter := func(prefix bool, i int, dest io.Writer) io.Writer {
		if prefix {
			targetName := cmdUtil.StripTimeout(rs.Targets[i])
			dest = writerUtils.GetPrefixedWriter(
				[]byte(fmt.Sprintf("%d-%s: ", i, targetName)),
				true,
				dest,
			)
		}
		return dest
	}

	for i, out := range rs.Outputs {
		if out == "-" {
			state.Out = append(state.Out, makeWriter(rs.PrefixOutput, i, os.Stdout))
			state.Err = append(state.Err, makeWriter(rs.PrefixOutput, i, os.Stderr))
			continue
		}
		file, err := os.Create(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't create output file %s - %v\n", out, err)
			os.Exit(1)
		}
		defer file.Close()
		errorFile := fmt.Sprintf("%s.error", out)
		errF, err := os.Create(errorFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't create error file file %s - %v\n", errorFile, err)
			os.Exit(1)
		}
		defer errF.Close()

		state.Out = append(state.Out, makeWriter(rs.PrefixOutput, i, file))
		state.Err = append(state.Err, makeWriter(rs.PrefixOutput, i, errF))
	}

	exitCode := subcommands.ExitSuccess

	// If there's no batch size set then it'll be the whole thing so we can use one loop below.
	if rs.BatchSize == 0 {
		rs.BatchSize = len(rs.Targets)
	}
	// Save original lists since we'll replace rs with sub slices
	output := state.Out
	errors := state.Err

	batchCnt := 1
	if len(rs.Targets) > 0 {
		// How many batches? Integer math truncates so we have to do one more for remainder.
		batchCnt = (len(rs.Targets)-1)/rs.BatchSize + 1
	}
	for i := 0; i < batchCnt; i++ {
		start, end := i*rs.BatchSize, rs.BatchSize*(i+1)
		if end > len(rs.Targets) {
			end = len(rs.Targets)
		}
		// Set up a connection to the sansshell-server (possibly via proxy).
		conn, err := proxy.DialContext(ctx, rs.Proxy, rs.Targets[start:end], ops...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not connect to proxy %q node(s) in batch %d: %v\n", rs.Proxy, i, err)
			os.Exit(1)
		}

		conn.AuthzDryRun = rs.AuthzDryRun

		if rs.EnableMPA {
			conn.UnaryInterceptors = []proxy.UnaryInterceptor{mpahooks.ProxyClientUnaryInterceptor(state, rs.EnableMethodWideMPA)}
			conn.StreamInterceptors = []proxy.StreamInterceptor{mpahooks.ProxyClientStreamInterceptor(state, rs.EnableMethodWideMPA)}
		}

		if len(rs.ClientUnaryInterceptors) > 0 {
			conn.UnaryInterceptors = append(conn.UnaryInterceptors, rs.ClientUnaryInterceptors...)
		}
		if len(rs.ClientStreamInterceptors) > 0 {
			conn.StreamInterceptors = append(conn.StreamInterceptors, rs.ClientStreamInterceptors...)
		}

		state.Conn = conn
		state.CredSource = rs.CredSource
		state.Out = output[start:end]
		state.Err = errors[start:end]
		if len(rs.Targets) == 0 {
			// Special case - if we're talking directly to the proxy, we have an output of size 1
			state.Out = output[:1]
			state.Err = errors[:1]
		}
		if subcommands.Execute(ctx, state) != subcommands.ExitSuccess {
			exitCode = subcommands.ExitFailure
		}
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing connection - %v\n", err)
		}
	}

	// Invoke the subcommand, passing the dialed connection object
	os.Exit(int(exitCode))
}
