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

// package client provides the client interface for 'service'
package client

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/google/subcommands"

	"github.com/Snowflake-Labs/sansshell/client"
	pb "github.com/Snowflake-Labs/sansshell/services/service"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

const subPackage = "service"

func init() {
	subcommands.Register(&serviceCmd{}, subPackage)
}

func setup(f *flag.FlagSet) *subcommands.Commander {
	c := client.SetupSubpackage(subPackage, f)
	initSystemTypes()
	c.Register(&actionCmd{action: pb.Action_ACTION_RESTART}, "")
	c.Register(&listCmd{}, "")
	c.Register(&actionCmd{action: pb.Action_ACTION_START}, "")
	c.Register(&statusCmd{}, "")
	c.Register(&actionCmd{action: pb.Action_ACTION_STOP}, "")
	return c
}

type serviceCmd struct{}

func (*serviceCmd) Name() string { return subPackage }
func (p *serviceCmd) Synopsis() string {
	return client.GenerateSynopsis(setup(flag.NewFlagSet("", flag.ContinueOnError)))
}
func (p *serviceCmd) Usage() string {
	return client.GenerateUsage(subPackage, p.Synopsis())
}
func (*serviceCmd) SetFlags(f *flag.FlagSet) {}

func (p *serviceCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	c := setup(f)
	return c.Execute(ctx, args...)
}

var systemTypes []string
var systemTypeHelp string

func initSystemTypes() {
	for k := range pb.SystemType_name {
		systemTypes = append(systemTypes, systemTypeString(pb.SystemType(k)))
	}
	sort.Strings(systemTypes)
	systemTypeHelp = fmt.Sprintf("The system type (one of: [%s])", strings.Join(systemTypes, ","))
}

func flagToSystemType(val string) (pb.SystemType, error) {
	v := fmt.Sprintf("SYSTEM_TYPE_%s", strings.ToUpper(val))
	i, ok := pb.SystemType_value[v]
	if !ok {
		return pb.SystemType_SYSTEM_TYPE_UNKNOWN, fmt.Errorf("no such system %s", v)
	}
	return pb.SystemType(i), nil
}

func systemTypeFlag(f *flag.FlagSet, p *string) {
	f.StringVar(p, "system-type", "systemd", systemTypeHelp)
}

func systemTypeString(t pb.SystemType) string {
	return strings.ToLower(strings.TrimPrefix(t.String(), "SYSTEM_TYPE_"))
}

func statusString(s pb.Status) string {
	return strings.ToLower(strings.TrimPrefix(s.String(), "STATUS_"))
}

type actionCmd struct {
	action     pb.Action
	systemType string
}

func (a *actionCmd) actionString() string {
	return strings.ToLower(strings.TrimPrefix(a.action.String(), "ACTION_"))
}

func (a *actionCmd) Name() string { return a.actionString() }

func (a *actionCmd) Synopsis() string {
	return fmt.Sprintf("%s a service", a.actionString())
}

func (a *actionCmd) Usage() string {
	as := a.actionString()
	return fmt.Sprintf(`%s [--system-type <type>] <service>:
    %s the specified service`, as, as)
}

func (a *actionCmd) SetFlags(f *flag.FlagSet) {
	systemTypeFlag(f, &a.systemType)
}

func (a *actionCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	as := a.actionString()
	state := args[0].(*util.ExecuteState)
	errWriter := subcommands.DefaultCommander.Error
	if f.NArg() == 0 {
		fmt.Fprintln(errWriter, "Please specify a service.")
		subcommands.DefaultCommander.ExplainCommand(errWriter, a)
		return subcommands.ExitUsageError
	}

	system, err := flagToSystemType(a.systemType)
	if err != nil {
		fmt.Fprintln(errWriter, err)
		subcommands.DefaultCommander.ExplainCommand(errWriter, a)
		return subcommands.ExitUsageError
	}

	serviceName := f.Args()[0]
	req := &pb.ActionRequest{
		SystemType:  system,
		ServiceName: serviceName,
		Action:      a.action,
	}

	c := pb.NewServiceClientProxy(state.Conn)
	respChan, err := c.ActionOneMany(ctx, req)
	if err != nil {
		fmt.Fprintf(errWriter, "error executing %s: %v\n", as, err)
		return subcommands.ExitFailure
	}

	// Error holding the last observed non-nil error, which will
	// determine the exit status of the command.
	// The contract with the proxy and 'many' functions requires
	// that we completely drain the response channel, so we cannot
	// return early here.
	// Note that this is only the last non-nil error, and previous
	// error values may be lost.
	var lastErr error
	for resp := range respChan {
		out := state.Out[resp.Index]
		output := fmt.Sprintf("[%s] %s %v: OK", systemTypeString(system), serviceName, as)
		if resp.Error != nil {
			lastErr = fmt.Errorf("target %s (%d) returned error %w", resp.Target, resp.Index, resp.Error)
			output = lastErr.Error()
		}
		if _, err := fmt.Fprintln(out, output); err != nil {
			lastErr = fmt.Errorf("target %s (%d) output write error %w", resp.Target, resp.Index, err)
		}
	}
	if lastErr != nil {
		fmt.Fprintf(errWriter, "%s finished with error %v\n", as, lastErr)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

type statusCmd struct {
	systemType string
}

func (*statusCmd) Name() string     { return "status" }
func (*statusCmd) Synopsis() string { return "retrieve service status" }
func (*statusCmd) Usage() string {
	return `status [--system-type <type>] <service>
    return the status of the specified service
  `
}
func (s *statusCmd) SetFlags(f *flag.FlagSet) {
	systemTypeFlag(f, &s.systemType)
}

func (s *statusCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	errWriter := subcommands.DefaultCommander.Error
	if f.NArg() == 0 {
		fmt.Fprintln(errWriter, "Please specify a service.")
		subcommands.DefaultCommander.ExplainCommand(errWriter, s)
		return subcommands.ExitUsageError
	}
	serviceName := f.Args()[0]

	system, err := flagToSystemType(s.systemType)
	if err != nil {
		fmt.Fprintln(errWriter, err)
		subcommands.DefaultCommander.ExplainCommand(errWriter, s)
		return subcommands.ExitUsageError
	}

	req := &pb.StatusRequest{
		SystemType:  system,
		ServiceName: serviceName,
	}
	c := pb.NewServiceClientProxy(state.Conn)

	respChan, err := c.StatusOneMany(ctx, req)

	if err != nil {
		fmt.Fprintf(errWriter, "error executing 'status' for service %s: %v\n", serviceName, err)
		return subcommands.ExitFailure
	}

	// Error holding the last observed non-nil error, which will
	// determine the exit status of the command.
	// The contract with the proxy and 'many' functions requires
	// that we completely drain the response channel, so we cannot
	// return early here.
	// Note that this is only the last non-nil error, and previous
	// error values may be lost.
	var lastErr error
	for resp := range respChan {
		out := state.Out[resp.Index]
		system, status := resp.Resp.GetSystemType(), resp.Resp.GetServiceStatus().GetStatus()
		output := fmt.Sprintf("[%s] %s : %s", systemTypeString(system), serviceName, statusString(status))
		if resp.Error != nil {
			lastErr = fmt.Errorf("target %s [%d] error: %w", resp.Target, resp.Index, resp.Error)
			output = lastErr.Error()
		}
		if _, err := fmt.Fprintln(out, output); err != nil {
			lastErr = fmt.Errorf("target %s [%d] write error: %w", resp.Target, resp.Index, err)
		}
	}
	if lastErr != nil {
		fmt.Fprintf(errWriter, "status finished with error: %v\n", lastErr)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

type listCmd struct {
	systemType string
}

func (*listCmd) Name() string     { return "services" }
func (*listCmd) Synopsis() string { return "list services with status" }
func (*listCmd) Usage() string {
	return `services [--system-type <type>]
    list services and their status
  `
}

func (l *listCmd) SetFlags(f *flag.FlagSet) {
	systemTypeFlag(f, &l.systemType)
}

func (l *listCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	errWriter := subcommands.DefaultCommander.Error

	system, err := flagToSystemType(l.systemType)
	if err != nil {
		fmt.Fprintln(errWriter, err)
		subcommands.DefaultCommander.ExplainCommand(errWriter, l)
		return subcommands.ExitUsageError
	}

	req := &pb.ListRequest{
		SystemType: system,
	}
	c := pb.NewServiceClientProxy(state.Conn)

	respChan, err := c.ListOneMany(ctx, req)

	if err != nil {
		fmt.Fprintf(errWriter, "error executing 'list' for services: %v\n", err)
		return subcommands.ExitFailure
	}

	// Error holding the last observed non-nil error, which will
	// determine the exit status of the command.
	// The contract with the proxy and 'many' functions requires
	// that we completely drain the response channel, so we cannot
	// return early here.
	// Note that this is only the last non-nil error, and previous
	// error values may be lost.
	var lastErr error
	for resp := range respChan {
		out := state.Out[resp.Index]
		if resp.Error != nil {
			lastErr = fmt.Errorf("target %s (%d) error: %w", resp.Target, resp.Index, resp.Error)
			if _, err := fmt.Fprintln(out, lastErr.Error()); err != nil {
				lastErr = fmt.Errorf("target %s (%d) writer error: %w", resp.Target, resp.Index, err)
			}
			continue
		}
		system := systemTypeString(resp.Resp.GetSystemType())
		for _, svc := range resp.Resp.Services {
			if _, err := fmt.Fprintf(out, "[%s] %s : %s\n", system, svc.GetServiceName(), statusString(svc.GetStatus())); err != nil {
				lastErr = fmt.Errorf("target %s [%d] writer error: %w", resp.Target, resp.Index, err)
			}
		}
	}
	if lastErr != nil {
		fmt.Fprintf(errWriter, "services finished with error: %v\n", lastErr)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
