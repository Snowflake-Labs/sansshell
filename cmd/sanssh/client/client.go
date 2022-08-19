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
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Snowflake-Labs/sansshell/auth/mtls"
	"github.com/Snowflake-Labs/sansshell/auth/opa/rpcauth"
	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	"github.com/google/subcommands"
	"google.golang.org/grpc"

	"github.com/Snowflake-Labs/sansshell/services/util"
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
	// Timeout is the duration to place on the context when making RPC calls.
	Timeout time.Duration
	// ClientPolicy is an optional OPA policy for determining outbound decisions.
	ClientPolicy string
	// PrefixOutput if true will prefix every line of output with '<index>-<target>: '
	PrefixOutput bool
	// BatchSize if non-zero will do the requested operation to the targets but in
	// N calls to the proxy where N is the target list size divided by BatchSize.
	BatchSize int
}

const (
	defaultOutput = "-"
)

type prefixWriter struct {
	prefix []byte
	start  bool
	dest   io.Writer
}

func (p *prefixWriter) Write(b []byte) (n int, err error) {
	// Keep track of the size of the incoming buf as we'll print more
	// but clients want to know we wrote what they asked for.
	tot := len(b)

	// If we just started emit a prefix
	if p.start {
		n, err = p.dest.Write(p.prefix)
		if err != nil {
			return n, err
		}
		p.start = false
	}

	// Find any newlines and augment them with the prefix appended onto them.

	// If the last byte is a newline we don't want to add a prefix yet as this may be the end of output.
	// Instead we'll remark start so the next one prints and remove the newline for now.
	if n := bytes.LastIndex(b, []byte{'\n'}); n == len(b)-1 {
		p.start = true
		b = b[:len(b)-1]
	}
	b = bytes.ReplaceAll(b, []byte{'\n'}, append(append([]byte{}, byte('\n')), p.prefix...))
	// If start got set above we need to add back the newline we dropped so output looks correct.
	// Thankfully b is now a new slice as Write() isn't supposed to directly modify the incoming one.
	if p.start {
		b = append(b, '\n')
	}
	n, err = p.dest.Write(b)
	if err != nil {
		return n, err
	}
	return tot, nil
}

// Run takes the given context and RunState and executes the command passed in after parsing with flags.
// As this is intended to be called from main() it doesn't return errors and will instead exit on any errors.
func Run(ctx context.Context, rs RunState) {
	// If we're running internal commands we don't need anything else.
	for _, f := range flag.Args() {
		switch f {
		case "help", "flags", "commands":
			os.Exit(int(subcommands.Execute(ctx, &util.ExecuteState{})))
		}
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
			rs.Outputs = append(rs.Outputs, filepath.Join(rs.OutputsDir, fmt.Sprintf("%d-%s", i, t)))
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

	var clientAuthz *rpcauth.Authorizer
	if rs.ClientPolicy != "" {
		clientAuthz, err = rpcauth.NewWithPolicy(ctx, rs.ClientPolicy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not load policy: %v\n", err)
		}
	}

	// We may need an option for doing client OPA checks.
	ops := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}
	if clientAuthz != nil {
		ops = append(ops, grpc.WithStreamInterceptor(clientAuthz.AuthorizeClientStream))
	}

	state := &util.ExecuteState{
		Dir: dir,
	}

	makeWriter := func(prefix bool, i int, dest io.Writer) io.Writer {
		if prefix {
			dest = &prefixWriter{
				start:  true,
				dest:   dest,
				prefix: []byte(fmt.Sprintf("%d-%s: ", i, rs.Targets[i])),
			}
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

	ctx, cancel := context.WithTimeout(ctx, rs.Timeout)
	defer cancel()

	exitCode := subcommands.ExitSuccess

	// If there's no batch size set then it'll be the whole thing so we can use one loop below.
	if rs.BatchSize == 0 {
		rs.BatchSize = len(rs.Targets)
	}
	// Save original lists since we'll replace rs with sub slices
	output := state.Out
	errors := state.Err

	batchCnt := 0
	if len(rs.Targets) > 0 {
		batchCnt = len(rs.Targets) / rs.BatchSize
	}
	// How many batches? Integer math truncates so we have to do one more after for remainder.
	for i := 0; i < batchCnt; i++ {
		// Set up a connection to the sansshell-server (possibly via proxy).
		conn, err := proxy.Dial(rs.Proxy, rs.Targets[i*rs.BatchSize:rs.BatchSize*(i+1)], ops...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not connect to proxy %q node(s) in batch %d: %v\n", rs.Proxy, i, err)
			os.Exit(1)
		}
		defer func() {
			if err := conn.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error closing connection - %v\n", err)
			}
		}()
		state.Conn = conn
		state.Out = output[i*rs.BatchSize : rs.BatchSize*(i+1)]
		state.Err = errors[i*rs.BatchSize : rs.BatchSize*(i+1)]
		if subcommands.Execute(ctx, state) != subcommands.ExitSuccess {
			exitCode = subcommands.ExitFailure
		}
	}

	// Remainder or the fall through case of no targets (i.e. a proxy command).
	if len(rs.Targets)-batchCnt*rs.BatchSize > 0 || len(rs.Targets) == 0 {
		conn, err := proxy.Dial(rs.Proxy, rs.Targets[batchCnt*rs.BatchSize:], ops...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not connect to proxy %q node(s) in last batch: %v\n", rs.Proxy, err)
			os.Exit(1)
		}
		defer func() {
			if err := conn.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error closing connection - %v\n", err)
			}
		}()
		state.Conn = conn
		state.Out = output[batchCnt*rs.BatchSize:]
		state.Err = errors[batchCnt*rs.BatchSize:]
		if subcommands.Execute(ctx, state) != subcommands.ExitSuccess {
			exitCode = subcommands.ExitFailure
		}
	}

	// Invoke the subcommand, passing the dialed connection object
	os.Exit(int(exitCode))
}
