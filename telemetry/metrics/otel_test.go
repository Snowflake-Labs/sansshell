/*
Copyright (c) 2023 Snowflake Inc. All rights reserved.

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
package metrics

import (
	"context"
	"testing"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func TestNewOtelRecorderWithInvalidOption(t *testing.T) {
	errInvalid := errors.New("invalid")
	invalidOption := func() Option {
		return optionFunc(func(m *OtelRecorder) error {
			return errInvalid
		})
	}

	_, err := NewOtelRecorder(global.Meter("sansshelltesting"), invalidOption())
	t.Log(err)
	testutil.FatalOnNoErr("expected NewOtelRecorder to return error", err, t)
}

func TestNewOtelRecorderWithMetricNamePrefixOption(t *testing.T) {
	metricPrefix := "test-prefix-"
	recorder, err := NewOtelRecorder(global.Meter("sansshelltesting"), WithMetricNamePrefix(metricPrefix))
	testutil.FatalOnErr("unexpected err on NewOtelRecorder", err, t)
	if recorder.prefix != metricPrefix {
		t.Fatalf("Option WithMetricNamePrefix was not applied. Expected recorder prefix: %v, got: %v", metricPrefix, recorder.prefix)
	}
}

func TestAddPrefix(t *testing.T) {
	metricName := "error_count"
	prefix := "test_prefix"
	want := prefix + "_" + metricName
	got := addPrefix(prefix, metricName)
	if got != want {
		t.Fatalf("addPrefix returns: %v, want: %v", got, want)
	}
}

func TestRegisterAndAddInt64Counter(t *testing.T) {
	recorder, err := NewOtelRecorder(global.Meter("sansshelltesting"))
	testutil.FatalOnErr("unexpected err on NewOtelRecorder", err, t)
	counterName := "error_count"
	errRegister := recorder.RegisterInt64Counter(counterName, "")
	testutil.FatalOnErr("unexpected err on RegisterInt64Counter", errRegister, t)

	// registering an existing metric shouldn't return an error
	errRegister = recorder.RegisterInt64Counter(counterName, "")
	testutil.FatalOnErr("unexpected err on RegisterInt64Counter", errRegister, t)

	// adding a registered metric shouldn't return an error
	errAdd := recorder.AddInt64Counter(context.Background(), counterName, 1)
	testutil.FatalOnErr("unexpected err on AddInt64Counter", errAdd, t)

	// adding an unregistered metric should return an error
	errAdd = recorder.AddInt64Counter(context.Background(), "metricdoesnotexist", 1)
	t.Log(err)
	testutil.FatalOnNoErr("expected AddInt64Counter to return an error", errAdd, t)
}

func TestRegisterInt64Gauge(t *testing.T) {
	recorder, err := NewOtelRecorder(global.Meter("sansshelltesting"))
	testutil.FatalOnErr("unexpected err on NewOtelRecorder", err, t)
	gaugeName := "disk_usage"
	diskUsage := int64(50)
	errRegister := recorder.RegisterInt64Gauge(gaugeName, "", func(_ context.Context, obsrv instrument.Int64Observer) error {
		obsrv.Observe(diskUsage)
		return nil
	})
	testutil.FatalOnErr("unexpected err on RegisterInt64Gauge", errRegister, t)

	// registering an existing metric shouldn't return an error
	errRegister = recorder.RegisterInt64Gauge(gaugeName, "diffdesc", func(_ context.Context, obsrv instrument.Int64Observer) error {
		obsrv.Observe(0)
		return nil
	})
	testutil.FatalOnErr("unexpected err on RegisterInt64Counter", errRegister, t)
}
