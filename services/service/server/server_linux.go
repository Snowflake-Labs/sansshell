//go:build linux
// +build linux

package server

import (
	"context"
	"sort"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Snowflake-Labs/sansshell/services/service"
)

// SystemD deals in 'units', which might be services, devices, sockets,
// or a variety of other types.
// Each unit has several associated fields which collectively describe
// its status.
// These are returned as strings from calls to systemd, and the values
// are defined here:
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

// SystemD operations are asynchronous, and typically return
// immediately with a job ID (or error). The ultimate status
// of the operation is eventually delivered as a string
// which describes the result.
const (
	resultDone = "done"
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
	ListUnitsContext(ctx context.Context) ([]dbus.UnitStatus, error)
	StartUnitContext(ctx context.Context, name string, mode string, ch chan<- string) (int, error)
	StopUnitContext(ctx context.Context, name string, mode string, ch chan<- string) (int, error)
	RestartUnitContext(ctx context.Context, name string, mode string, ch chan<- string) (int, error)
	Close()
}

// a server implements pb.ServiceServer
type server struct {
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

// See: pb.ServiceServer.List
func (s *server) List(ctx context.Context, req *pb.ListRequest) (*pb.ListReply, error) {
	switch req.SystemType {
	case pb.SystemType_SYSTEM_TYPE_UNKNOWN, pb.SystemType_SYSTEM_TYPE_SYSTEMD:
		// unknown and systemd both map to systemd, which we support
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported system type %s", req.SystemType)
	}

	conn, err := s.dialSystemd(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error establishing systemd connection: %v", err)
	}
	defer conn.Close()

	units, err := conn.ListUnitsContext(ctx)
	if err != nil {
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
	switch req.SystemType {
	case pb.SystemType_SYSTEM_TYPE_UNKNOWN, pb.SystemType_SYSTEM_TYPE_SYSTEMD:
		// unknown and systemd both map to systemd, which we support
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported system type %s", req.SystemType)
	}

	unitName := req.GetServiceName()
	if len(unitName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	// Accept either 'foo' or 'foo.service'
	if !strings.HasSuffix(unitName, unitSuffixService) {
		unitName = unitName + unitSuffixService
	}

	conn, err := s.dialSystemd(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error establishing systemd connection: %v", err)
	}
	defer conn.Close()

	// NB: ideally we'd use ListUnitsByNamesContext, but older versions of systemd
	// do not support this method, so the most failsafe method that works on all systemd
	// versions is to retrieve the full list of units, and filter here.
	units, err := conn.ListUnitsContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "systemd status error %v", err)
	}
	for _, u := range units {
		if u.Name == unitName {
			return &pb.StatusReply{
				SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceStatus: &pb.ServiceStatus{
					ServiceName: req.GetServiceName(),
					Status:      unitStateToStatus(u),
				},
			}, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "service %s was not found", req.GetServiceName())
}

// See: pb.ServiceServer.Action
func (s *server) Action(ctx context.Context, req *pb.ActionRequest) (*pb.ActionReply, error) {
	switch req.SystemType {
	case pb.SystemType_SYSTEM_TYPE_UNKNOWN, pb.SystemType_SYSTEM_TYPE_SYSTEMD:
		// unknown and systemd both map to systemd, which we support
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported system type %s", req.SystemType)
	}

	unitName := req.GetServiceName()
	if len(unitName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}
	// Accept either 'foo' or 'foo.service'
	if !strings.HasSuffix(unitName, unitSuffixService) {
		unitName = unitName + unitSuffixService
	}

	conn, err := s.dialSystemd(ctx)
	if err != nil {
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
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid action type %v", req.Action)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error performing action %v: %v", req.Action, err)
	}

	// NB: delivery of a value on resultchan respects context cancellation, and will
	// deliver a value of 'cancelled' if the ctx is cancelled by a client disconnect,
	// so it's safe to do a simple recv.
	result := <-resultChan
	if result != resultDone {
		return nil, status.Errorf(codes.Internal, "error performing action %v: %v", req.Action, result)
	}
	return &pb.ActionReply{
		SystemType:  pb.SystemType_SYSTEM_TYPE_SYSTEMD,
		ServiceName: req.GetServiceName(),
	}, nil
}
