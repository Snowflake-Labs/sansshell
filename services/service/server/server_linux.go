//go:build linux
// +build linux

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
	"encoding/json"
	"sort"
	"strings"
	"syscall"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Snowflake-Labs/sansshell/services/service"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
)

// Systemd deals in 'units', which might be services, devices, sockets,
// or a variety of other types.
// Each unit has several associated fields which collectively describe
// its status.
// These are returned as strings from calls to systemd, and the canonical
// values are defined in systemd here:
// https://github.com/systemd/systemd/blob/b09869eaf704a9fa0e90fbb2fde52b0ce7769e37/src/basic/unit-def.c

// A unit's load state indicates the state of the unit's configuration
// with respect to systemd.
// Only a subset of the possible values which are of interest to us
// are defined here.
const (
	// systemd has loaded the unit definition into memory
	loadStateLoaded = "loaded"
)

// A unit's active state describes the administrative state of the unit
// (e.g. whether it is enabled or disabled)
// Only a subset of the possible values are defined here.
const (
	activeStateActive = "active"
)

// A unit's sub-state provides more granular status of the unit
// that is specific to a particular unit type. In this case,
// we are interested only in substates relevant to the 'service'
// type.
const (
	substateRunning = "running"
)

// The suffix used for units of type 'service'
const (
	unitSuffixService = ".service"
)

// SystemD operations on units can take several 'modes', which
// determine how the operation should interact with other
// in-flight operations, or an operation's effect on other
// units.
const (
	// replace supplants any other in-progress operation of the
	// same type, and iteracts normally with dependent units (
	// e.g. starting them if necessary).
	modeReplace = "replace"
)

// Operations on units are asynchronous, and typically return
// immediately with a job ID (or error). The ultimate status
// of the operation is eventually delivered as a string
// which describes the result.
const (
	operationResultDone = "done"
)

// Metrics
var (
	serviceListFailureCounter = metrics.MetricDefinition{Name: "actions_service_list_failure",
		Description: "number of failures when performing service.List"}
	serviceStatusFailureCounter = metrics.MetricDefinition{Name: "actions_service_status_failure",
		Description: "number of failures when performing service.Status"}
	serviceActionFailureCounter = metrics.MetricDefinition{Name: "actions_service_action_failure",
		Description: "number of failures when performing service.Action"}
)

// convert a dbus.UnitStatus to a servicepb.Status
func unitStateToStatus(u dbus.UnitStatus) pb.Status {
	switch {
	// A service is 'running' if it's loaded, active, and running.
	case u.LoadState == loadStateLoaded && u.ActiveState == activeStateActive && u.SubState == substateRunning:
		return pb.Status_STATUS_RUNNING
		// Otherwise, as long as the unit definition is loaded into memory, it's stopped.
	case u.LoadState == loadStateLoaded:
		return pb.Status_STATUS_STOPPED
		// If the unit definition is not loaded, the status is unknown.
	default:
		return pb.Status_STATUS_UNKNOWN
	}
}

// a subset of dbus.Conn used to mock for testing
type systemdConnection interface {
	GetUnitPropertiesContext(ctx context.Context, unit string) (map[string]interface{}, error)
	ListUnitsContext(ctx context.Context) ([]dbus.UnitStatus, error)
	StartUnitContext(ctx context.Context, name string, mode string, ch chan<- string) (int, error)
	StopUnitContext(ctx context.Context, name string, mode string, ch chan<- string) (int, error)
	RestartUnitContext(ctx context.Context, name string, mode string, ch chan<- string) (int, error)
	DisableUnitFilesContext(ctx context.Context, files []string, runtime bool) ([]dbus.DisableUnitFileChange, error)
	EnableUnitFilesContext(ctx context.Context, files []string, runtime bool, force bool) (bool, []dbus.EnableUnitFileChange, error)
	KillUnitContext(ctx context.Context, name string, signal int32)
	ReloadContext(ctx context.Context) error
	Close()
}

// a server implements pb.ServiceServer
type server struct {
	// dialSystemd is the function used to create connections to systemd.
	dialSystemd func(context.Context) (systemdConnection, error)
}

func dialSystemd(ctx context.Context) (systemdConnection, error) {
	conn, err := dbus.NewSystemdConnectionContext(ctx)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func createServer() pb.ServiceServer {
	return &server{dialSystemd: dialSystemd}
}

// implement sort.Interface for UnitStatus slices, so that List can return
// services in sorted order.
type byName []dbus.UnitStatus

func (s byName) Len() int           { return len(s) }
func (s byName) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func checkSupportedSystem(t pb.SystemType) error {
	switch t {
	case pb.SystemType_SYSTEM_TYPE_UNKNOWN, pb.SystemType_SYSTEM_TYPE_SYSTEMD:
		return nil
	default:
		return status.Errorf(codes.InvalidArgument, "unsupported system type %s", t)
	}
}

// See: pb.ServiceServer.List
func (s *server) List(ctx context.Context, req *pb.ListRequest) (*pb.ListReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if err := checkSupportedSystem(req.SystemType); err != nil {
		return nil, err
	}

	conn, err := s.dialSystemd(ctx)
	if err != nil {
		recorder.CounterOrLog(ctx, serviceListFailureCounter, 1, attribute.String("reason", "dial_systemd_err"))
		return nil, status.Errorf(codes.Internal, "error establishing systemd connection: %v", err)
	}
	defer conn.Close()

	units, err := conn.ListUnitsContext(ctx)
	if err != nil {
		recorder.CounterOrLog(ctx, serviceListFailureCounter, 1, attribute.String("reason", "list_units_err"))
		return nil, status.Errorf(codes.Internal, "systemd list error %v", err)
	}
	sort.Sort(byName(units))

	resp := &pb.ListReply{
		SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
	}

	for _, u := range units {
		// we're only interested in 'service' units.
		// Newer version of SystemD support the ListUnitsPatterns dbus method,
		// but this is not guaranteed to be present, so we do the filtering
		// here.
		if !strings.HasSuffix(u.Name, unitSuffixService) {
			continue
		}
		resp.Services = append(resp.Services, &pb.ServiceStatus{
			ServiceName: strings.TrimSuffix(u.Name, unitSuffixService),
			Status:      unitStateToStatus(u),
		})
	}
	return resp, nil
}

// See: pb.ServiceServer.Status
func (s *server) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	logger := logr.FromContextOrDiscard(ctx)
	if err := checkSupportedSystem(req.SystemType); err != nil {
		return nil, err
	}

	unitName := req.GetServiceName()
	if len(unitName) == 0 {
		recorder.CounterOrLog(ctx, serviceStatusFailureCounter, 1, attribute.String("reason", "missing_name"))
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	// Accept either 'foo' or 'foo.service'
	if !strings.HasSuffix(unitName, unitSuffixService) {
		unitName = unitName + unitSuffixService
	}

	conn, err := s.dialSystemd(ctx)
	if err != nil {
		recorder.CounterOrLog(ctx, serviceStatusFailureCounter, 1, attribute.String("reason", "dial_systemd_err"))
		return nil, status.Errorf(codes.Internal, "error establishing systemd connection: %v", err)
	}
	defer conn.Close()

	properties, err := conn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		recorder.CounterOrLog(ctx, serviceStatusFailureCounter, 1, attribute.String("reason", "get_unit_properties_err"))
		logger.V(3).Info("GetUnitPropertiesContext err: " + err.Error())
		return nil, status.Errorf(codes.Internal, "failed to get unit properties of %s", req.GetServiceName())
	}
	// cast map[string]interface{} to dbus.UnitStatus{} using json marshal + unmarshal
	propertiesJson, err := json.Marshal(properties)
	if err != nil {
		recorder.CounterOrLog(ctx, serviceStatusFailureCounter, 1, attribute.String("reason", "json_marshal_err"))
		logger.V(3).Info("failed to marshal properties: " + err.Error())
		return nil, status.Errorf(codes.Internal, "failed to marshal unit properties to json")
	}
	unitState := dbus.UnitStatus{}
	if errUnmarshal := json.Unmarshal(propertiesJson, &unitState); errUnmarshal != nil {
		recorder.CounterOrLog(ctx, serviceStatusFailureCounter, 1, attribute.String("reason", "json_unmarshal_err"))
		logger.V(3).Info("failed to unmarshal properties: " + errUnmarshal.Error())
		return nil, status.Errorf(codes.Internal, "failed to unmarshal unit properties to json")
	}
	return &pb.StatusReply{
		SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
		ServiceStatus: &pb.ServiceStatus{
			ServiceName: req.GetServiceName(),
			Status:      unitStateToStatus(unitState),
		},
	}, nil
}

// See: pb.ServiceServer.Action
func (s *server) Action(ctx context.Context, req *pb.ActionRequest) (*pb.ActionReply, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if err := checkSupportedSystem(req.SystemType); err != nil {
		recorder.CounterOrLog(ctx, serviceActionFailureCounter, 1, attribute.String("reason", "not_supported"))
		return nil, err
	}

	unitName := req.GetServiceName()
	if len(unitName) == 0 {
		recorder.CounterOrLog(ctx, serviceActionFailureCounter, 1, attribute.String("reason", "missing_name"))
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}
	// Accept either 'foo' or 'foo.service'
	if !strings.HasSuffix(unitName, unitSuffixService) {
		unitName = unitName + unitSuffixService
	}

	conn, err := s.dialSystemd(ctx)
	if err != nil {
		recorder.CounterOrLog(ctx, serviceActionFailureCounter, 1, attribute.String("reason", "dial_systemd_err"))
		return nil, status.Errorf(codes.Internal, "error establishing systemd connection: %v", err)
	}
	defer conn.Close()

	resultChan := make(chan string)
	switch req.Action {
	case pb.Action_ACTION_START:
		_, err = conn.StartUnitContext(ctx, unitName, modeReplace, resultChan)
	case pb.Action_ACTION_RESTART:
		_, err = conn.RestartUnitContext(ctx, unitName, modeReplace, resultChan)
	case pb.Action_ACTION_STOP:
		_, err = conn.StopUnitContext(ctx, unitName, modeReplace, resultChan)
	case pb.Action_ACTION_ENABLE:
		_, _, err = conn.EnableUnitFilesContext(ctx, []string{unitName}, false, true)
	case pb.Action_ACTION_DISABLE:
		_, err = conn.DisableUnitFilesContext(ctx, []string{unitName}, false)
	case pb.Action_ACTION_KILL:
		conn.KillUnitContext(ctx, unitName, int32(syscall.SIGKILL))
	default:
		recorder.CounterOrLog(ctx, serviceActionFailureCounter, 1, attribute.String("reason", "invalid_action"))
		return nil, status.Errorf(codes.InvalidArgument, "invalid action type %v", req.Action)
	}
	if err != nil {
		recorder.CounterOrLog(ctx, serviceActionFailureCounter, 1, attribute.String("reason", "action_err"))
		return nil, status.Errorf(codes.Internal, "error performing action %v: %v", req.Action, err)
	}

	// NB: delivery of a value on resultchan respects context cancellation, and will
	// deliver a value of 'cancelled' if the ctx is cancelled by a client disconnect,
	// so it's safe to do a simple recv.
	// Enable/disable don't use this method so we skip the channel (since it would hang)
	// and instead force a reload which is what systemctl does when it enables/disables.
	switch req.Action {
	case pb.Action_ACTION_START, pb.Action_ACTION_RESTART, pb.Action_ACTION_STOP:
		result := <-resultChan
		if result != operationResultDone {
			recorder.CounterOrLog(ctx, serviceActionFailureCounter, 1, attribute.String("reason", "action_err"))
			return nil, status.Errorf(codes.Internal, "error performing action %v: %v", req.Action, result)
		}
	case pb.Action_ACTION_ENABLE, pb.Action_ACTION_DISABLE:
		if err := conn.ReloadContext(ctx); err != nil {
			recorder.CounterOrLog(ctx, serviceActionFailureCounter, 1, attribute.String("reason", "reload_err"))
			return nil, status.Errorf(codes.Internal, "error reloading: %v", err)
		}
	case pb.Action_ACTION_KILL:
	default:
		recorder.CounterOrLog(ctx, serviceActionFailureCounter, 1, attribute.String("reason", "invalid_action"))
		return nil, status.Errorf(codes.InvalidArgument, "invalid action type %v for post actions", req.Action)
	}

	return &pb.ActionReply{
		SystemType:  pb.SystemType_SYSTEM_TYPE_SYSTEMD,
		ServiceName: req.GetServiceName(),
	}, nil
}
