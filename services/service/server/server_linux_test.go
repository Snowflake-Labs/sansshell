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
	"errors"
	"strings"
	"testing"

	"github.com/coreos/go-systemd/v22/dbus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Snowflake-Labs/sansshell/services/service"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

// errConn is a systemConnection that returns errors for all methods
type errConn string

func (e errConn) GetUnitPropertiesContext(ctx context.Context, unit string) (map[string]interface{}, error) {
	return nil, errors.New(string(e))
}

func (e errConn) ListUnitsContext(context.Context) ([]dbus.UnitStatus, error) {
	return nil, errors.New(string(e))
}
func (e errConn) StartUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, errors.New(string(e))
}
func (e errConn) StopUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, errors.New(string(e))
}
func (e errConn) RestartUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, errors.New(string(e))
}
func (e errConn) DisableUnitFilesContext(context.Context, []string, bool) ([]dbus.DisableUnitFileChange, error) {
	return nil, errors.New(string(e))
}
func (e errConn) EnableUnitFilesContext(context.Context, []string, bool, bool) (bool, []dbus.EnableUnitFileChange, error) {
	return false, nil, errors.New(string(e))
}
func (e errConn) ReloadContext(ctx context.Context) error {
	return errors.New(string(e))
}
func (errConn) Close() {}

func TestDialError(t *testing.T) {
	sentinel := errors.New("dial error")
	s := &server{
		dialSystemd: func(context.Context) (systemdConnection, error) { return nil, sentinel },
	}
	t.Run("list", func(t *testing.T) {
		t.Parallel()
		_, err := s.List(context.Background(), &pb.ListRequest{})

		if status.Code(err) != codes.Internal || !strings.Contains(err.Error(), sentinel.Error()) {
			t.Errorf("err was %v, want internal error with message containing %v", err, sentinel)
		}
	})
	t.Run("status", func(t *testing.T) {
		t.Parallel()
		_, err := s.Status(context.Background(), &pb.StatusRequest{ServiceName: "foo"})
		if status.Code(err) != codes.Internal || !strings.Contains(err.Error(), sentinel.Error()) {
			t.Errorf("err was %v, want internal error with message containing %v", err, sentinel)
		}
	})
	t.Run("action", func(t *testing.T) {
		t.Parallel()
		_, err := s.Action(context.Background(), &pb.ActionRequest{Action: pb.Action_ACTION_START, ServiceName: "foo"})
		if status.Code(err) != codes.Internal || !strings.Contains(err.Error(), sentinel.Error()) {
			t.Errorf("err was %v, want internal error with message containing %v", err, sentinel)
		}
	})
}

var (
	notImplementedError = status.Error(codes.Unimplemented, "")
)

type listConn []dbus.UnitStatus

func (l listConn) GetUnitPropertiesContext(ctx context.Context, unit string) (map[string]interface{}, error) {
	return nil, notImplementedError
}
func (l listConn) ListUnitsContext(context.Context) ([]dbus.UnitStatus, error) {
	return l, nil
}
func (l listConn) StartUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, notImplementedError
}
func (l listConn) StopUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, notImplementedError
}
func (l listConn) RestartUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, notImplementedError
}
func (l listConn) DisableUnitFilesContext(context.Context, []string, bool) ([]dbus.DisableUnitFileChange, error) {
	return nil, notImplementedError
}
func (l listConn) EnableUnitFilesContext(context.Context, []string, bool, bool) (bool, []dbus.EnableUnitFileChange, error) {
	return false, nil, notImplementedError
}
func (l listConn) ReloadContext(ctx context.Context) error {
	return notImplementedError
}
func (listConn) Close() {}

func wantStatusErr(code codes.Code, message string) func(string, error, *testing.T) {
	return func(op string, e error, t *testing.T) {
		t.Helper()
		if status.Code(e) != code || !strings.Contains(e.Error(), message) {
			t.Fatalf("%s err was %v, want status with code %v and message '%s'", op, e, code, message)
		}
	}
}

func TestList(t *testing.T) {
	for _, tc := range []struct {
		name    string
		conn    systemdConnection
		req     *pb.ListRequest
		want    *pb.ListReply
		errFunc func(string, error, *testing.T)
	}{
		{
			name:    "list error",
			conn:    errConn("sentinel"),
			req:     &pb.ListRequest{},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "sentinel"),
		},
		{
			name: "empty response",
			conn: listConn([]dbus.UnitStatus{}),
			req:  &pb.ListRequest{},
			want: &pb.ListReply{
				SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
			},
			errFunc: testutil.FatalOnErr,
		},
		{
			name: "bad system type",
			conn: listConn([]dbus.UnitStatus{}),
			req: &pb.ListRequest{
				SystemType: pb.SystemType(5),
			},
			want:    nil,
			errFunc: wantStatusErr(codes.InvalidArgument, "unsupported system"),
		},
		{
			name: "output sorted and filtered",
			conn: listConn([]dbus.UnitStatus{
				{
					Name:        "foo.service",
					LoadState:   loadStateLoaded,
					ActiveState: activeStateActive,
					SubState:    substateRunning,
				},
				{
					Name:        "bar.service",
					LoadState:   loadStateLoaded,
					ActiveState: activeStateActive,
					SubState:    substateRunning,
				},
				{
					Name:        "baz.socket",
					LoadState:   loadStateLoaded,
					ActiveState: activeStateActive,
					SubState:    substateRunning,
				},
			}),
			req: &pb.ListRequest{},
			want: &pb.ListReply{
				SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				Services: []*pb.ServiceStatus{
					{
						ServiceName: "bar",
						Status:      pb.Status_STATUS_RUNNING,
					},
					{
						ServiceName: "foo",
						Status:      pb.Status_STATUS_RUNNING,
					},
				},
			},
			errFunc: testutil.FatalOnErr,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &server{
				dialSystemd: func(context.Context) (systemdConnection, error) {
					return tc.conn, nil
				},
			}
			got, err := s.List(context.Background(), tc.req)
			tc.errFunc("List", err, t)
			testutil.DiffErr(tc.name, got, tc.want, t)
		})
	}
}

type getConn []map[string]interface{}

func (g getConn) GetUnitPropertiesContext(ctx context.Context, unit string) (map[string]interface{}, error) {
	return nil, nil
}
func (g getConn) ListUnitsContext(context.Context) ([]dbus.UnitStatus, error) {
	return nil, nil
}
func (g getConn) StartUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, notImplementedError
}
func (g getConn) StopUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, notImplementedError
}
func (g getConn) RestartUnitContext(context.Context, string, string, chan<- string) (int, error) {
	return 0, notImplementedError
}
func (g getConn) DisableUnitFilesContext(context.Context, []string, bool) ([]dbus.DisableUnitFileChange, error) {
	return nil, notImplementedError
}
func (g getConn) EnableUnitFilesContext(context.Context, []string, bool, bool) (bool, []dbus.EnableUnitFileChange, error) {
	return false, nil, notImplementedError
}
func (g getConn) ReloadContext(ctx context.Context) error {
	return notImplementedError
}
func (getConn) Close() {}

func TestStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		conn    systemdConnection
		req     *pb.StatusRequest
		want    *pb.StatusReply
		errFunc func(string, error, *testing.T)
	}{
		{
			name: "status error",
			conn: errConn("sentinel"),
			req: &pb.StatusRequest{
				ServiceName: "foo",
			},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "sentinel"),
		},
		{
			name:    "missing service",
			conn:    errConn("not returned"),
			req:     &pb.StatusRequest{},
			want:    nil,
			errFunc: wantStatusErr(codes.InvalidArgument, "service name"),
		},
		{
			name: "bad system",
			conn: errConn("not returned"),
			req: &pb.StatusRequest{
				SystemType: pb.SystemType(5),
			},
			want:    nil,
			errFunc: wantStatusErr(codes.InvalidArgument, "system"),
		},
		{
			name: "not found",
			conn: getConn([]map[string]interface{}{}),
			req: &pb.StatusRequest{
				ServiceName: "foo",
			},
			want: &pb.StatusReply{
				SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceStatus: &pb.ServiceStatus{
					ServiceName: "foo",
					Status:      pb.Status_STATUS_UNKNOWN,
				},
			},
			errFunc: testutil.FatalOnErr,
		},
		{
			name: "accept with service suffix",
			conn: getConn([]map[string]interface{}{
				{
					"Name":        "foo.service",
					"LoadState":   loadStateLoaded,
					"ActiveState": activeStateActive,
					"SubState":    substateRunning,
				},
				{
					"Name":        "bar.service",
					"LoadState":   loadStateLoaded,
					"ActiveState": activeStateActive,
					"SubState":    substateRunning,
				},
			}),
			req: &pb.StatusRequest{
				ServiceName: "foo.service",
			},
			want: &pb.StatusReply{
				SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceStatus: &pb.ServiceStatus{
					ServiceName: "foo.service",
					Status:      pb.Status_STATUS_RUNNING,
				},
			},
			errFunc: testutil.FatalOnErr,
		},
		{
			name: "accept without service suffix",
			conn: getConn([]map[string]interface{}{
				{
					"Name":        "foo.service",
					"LoadState":   loadStateLoaded,
					"ActiveState": activeStateActive,
					"SubState":    substateRunning,
				},
				{
					"Name":        "bar.service",
					"LoadState":   loadStateLoaded,
					"ActiveState": activeStateActive,
					"SubState":    substateRunning,
				},
			}),
			req: &pb.StatusRequest{
				ServiceName: "foo",
			},
			want: &pb.StatusReply{
				SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceStatus: &pb.ServiceStatus{
					ServiceName: "foo",
					Status:      pb.Status_STATUS_RUNNING,
				},
			},
			errFunc: testutil.FatalOnErr,
		},
		{
			name: "stopped service",
			conn: getConn([]map[string]interface{}{
				{
					"Name":        "foo.service",
					"LoadState":   loadStateLoaded,
					"ActiveState": activeStateActive,
					"SubState":    "dead",
				},
				{
					"Name":        "bar.service",
					"LoadState":   loadStateLoaded,
					"ActiveState": activeStateActive,
					"SubState":    substateRunning,
				},
			}),
			req: &pb.StatusRequest{
				ServiceName: "foo",
			},
			want: &pb.StatusReply{
				SystemType: pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceStatus: &pb.ServiceStatus{
					ServiceName: "foo",
					Status:      pb.Status_STATUS_STOPPED,
				},
			},
			errFunc: testutil.FatalOnErr,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &server{
				dialSystemd: func(context.Context) (systemdConnection, error) {
					return tc.conn, nil
				},
			}
			got, err := s.Status(context.Background(), tc.req)
			tc.errFunc("Status", err, t)
			testutil.DiffErr(tc.name, got, tc.want, t)
		})
	}
}

// actionConn is a systemd connection that delivers a given string as the
// final result of all actions
type actionConn struct {
	action    string
	err       error
	reloadErr error
}

func (a actionConn) GetUnitPropertiesContext(ctx context.Context, unit string) (map[string]interface{}, error) {
	return nil, nil
}
func (a actionConn) ListUnitsContext(context.Context) ([]dbus.UnitStatus, error) {
	return nil, nil
}
func (a actionConn) StartUnitContext(ctx context.Context, name string, mode string, c chan<- string) (int, error) {
	go func() {
		c <- string(a.action)
	}()
	return 1, a.err
}
func (a actionConn) StopUnitContext(ctx context.Context, name string, mode string, c chan<- string) (int, error) {
	go func() {
		c <- string(a.action)
	}()
	return 1, a.err
}
func (a actionConn) RestartUnitContext(ctx context.Context, name string, mode string, c chan<- string) (int, error) {
	go func() {
		c <- string(a.action)
	}()
	return 1, a.err
}
func (a actionConn) DisableUnitFilesContext(context.Context, []string, bool) ([]dbus.DisableUnitFileChange, error) {
	return []dbus.DisableUnitFileChange{
		{
			Type:        string(a.action),
			Filename:    string(a.action),
			Destination: string(a.action),
		},
	}, a.err
}
func (a actionConn) EnableUnitFilesContext(context.Context, []string, bool, bool) (bool, []dbus.EnableUnitFileChange, error) {
	return false, []dbus.EnableUnitFileChange{
		{
			Type:        string(a.action),
			Filename:    string(a.action),
			Destination: string(a.action),
		},
	}, a.err
}
func (a actionConn) ReloadContext(ctx context.Context) error {
	return a.reloadErr
}
func (actionConn) Close() {}

func TestAction(t *testing.T) {
	for _, tc := range []struct {
		name    string
		conn    systemdConnection
		req     *pb.ActionRequest
		want    *pb.ActionReply
		errFunc func(string, error, *testing.T)
	}{
		{
			name: "action error",
			conn: errConn("sentinel"),
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_START,
			},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "sentinel"),
		},
		{
			name: "missing service",
			conn: errConn("not returned"),
			req: &pb.ActionRequest{
				Action: pb.Action_ACTION_START,
			},
			want:    nil,
			errFunc: wantStatusErr(codes.InvalidArgument, "service name"),
		},
		{
			name: "missing action",
			conn: errConn("not returned"),
			req: &pb.ActionRequest{
				ServiceName: "foo",
			},
			want:    nil,
			errFunc: wantStatusErr(codes.InvalidArgument, "action"),
		},
		{
			name: "bad system",
			conn: errConn("not returned"),
			req: &pb.ActionRequest{
				SystemType: pb.SystemType(5),
			},
			want:    nil,
			errFunc: wantStatusErr(codes.InvalidArgument, "system"),
		},
		{
			name: "bad action",
			conn: errConn("not returned"),
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action(-1),
			},
			want:    nil,
			errFunc: wantStatusErr(codes.InvalidArgument, "action"),
		},
		{
			name: "start failed",
			conn: actionConn{"failed", nil, nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_START,
			},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "error performing action"),
		},
		{
			name: "stop failed",
			conn: actionConn{"failed", nil, nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_STOP,
			},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "error performing action"),
		},
		{
			name: "restart failed",
			conn: actionConn{"failed", nil, nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_RESTART,
			},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "error performing action"),
		},
		{
			name: "enable failed",
			conn: actionConn{operationResultDone, errors.New("error"), nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_ENABLE,
			},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "error performing action"),
		},
		{
			name: "disable failed",
			conn: actionConn{operationResultDone, errors.New("error"), nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_DISABLE,
			},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "error performing action"),
		},
		{
			name: "disable failed due to reload fail",
			conn: actionConn{operationResultDone, nil, errors.New("error")},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_DISABLE,
			},
			want:    nil,
			errFunc: wantStatusErr(codes.Internal, "error reloading"),
		},
		{
			name: "start success",
			conn: actionConn{operationResultDone, nil, nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_START,
			},
			want: &pb.ActionReply{
				SystemType:  pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceName: "foo",
			},
			errFunc: testutil.FatalOnErr,
		},
		{
			name: "stop success",
			conn: actionConn{operationResultDone, nil, nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_STOP,
			},
			want: &pb.ActionReply{
				SystemType:  pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceName: "foo",
			},
			errFunc: testutil.FatalOnErr,
		},
		{
			name: "restart success",
			conn: actionConn{operationResultDone, nil, nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_RESTART,
			},
			want: &pb.ActionReply{
				SystemType:  pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceName: "foo",
			},
			errFunc: testutil.FatalOnErr,
		},
		{
			name: "enable success",
			conn: actionConn{operationResultDone, nil, nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_ENABLE,
			},
			want: &pb.ActionReply{
				SystemType:  pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceName: "foo",
			},
			errFunc: testutil.FatalOnErr,
		},
		{
			name: "disable success",
			conn: actionConn{operationResultDone, nil, nil},
			req: &pb.ActionRequest{
				ServiceName: "foo",
				Action:      pb.Action_ACTION_DISABLE,
			},
			want: &pb.ActionReply{
				SystemType:  pb.SystemType_SYSTEM_TYPE_SYSTEMD,
				ServiceName: "foo",
			},
			errFunc: testutil.FatalOnErr,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &server{
				dialSystemd: func(context.Context) (systemdConnection, error) {
					return tc.conn, nil
				},
			}
			got, err := s.Action(context.Background(), tc.req)
			tc.errFunc("Action", err, t)
			testutil.DiffErr(tc.name, got, tc.want, t)
		})
	}
}
