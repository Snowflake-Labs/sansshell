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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

func TestNewOtelRecorderWithInvalidOption(t *testing.T) {
	errInvalid := errors.New("invalid")
	invalidOption := func() Option {
		return optionFunc(func(m *OtelRecorder) error {
			return errInvalid
		})
	}

	_, err := NewOtelRecorder(otel.Meter("sansshelltesting"), invalidOption())
	t.Log(err)
	testutil.FatalOnNoErr("expected NewOtelRecorder to return error", err, t)
}

func TestNewOtelRecorderWithMetricNamePrefixOption(t *testing.T) {
	metricPrefix := "test-prefix-"
	recorder, err := NewOtelRecorder(otel.Meter("sansshelltesting"), WithMetricNamePrefix(metricPrefix))
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

func TestAddInt64Counter(t *testing.T) {
	recorder, err := NewOtelRecorder(otel.Meter("sansshelltesting"))
	testutil.FatalOnErr("unexpected err on NewOtelRecorder", err, t)
	counterDef := MetricDefinition{Name: "test_counter", Description: "test"}
	ctx := context.Background()
	// recording a counter for the first time shouldn't return an error
	errCounter := recorder.Counter(ctx, counterDef, 1)
	testutil.FatalOnErr("unexpected err on RegisterInt64Counter", errCounter, t)

	// recording the counter subsequently shouldn't return an error
	errCounter = recorder.Counter(ctx, counterDef, 1)
	testutil.FatalOnErr("unexpected err on RegisterInt64Counter", errCounter, t)
}

func TestRegisterInt64Gauge(t *testing.T) {
	recorder, err := NewOtelRecorder(otel.Meter("sansshelltesting"))
	testutil.FatalOnErr("unexpected err on NewOtelRecorder", err, t)
	gaugeDef := MetricDefinition{Name: "disk_usage", Description: "disk"}
	ctx := context.Background()
	diskUsage := int64(50)
	// recording a gauge for the first time shouldn't return an error
	errRegister := recorder.Gauge(ctx, gaugeDef, metric.Int64Callback(func(_ context.Context, obsrv metric.Int64Observer) error {
		obsrv.Observe(diskUsage)
		return nil
	}))
	testutil.FatalOnErr("unexpected err on RegisterInt64Gauge", errRegister, t)

	// recording a gauge subsequently shouldn't return an error
	errRegister = recorder.Gauge(ctx, gaugeDef, metric.Int64Callback(func(_ context.Context, obsrv metric.Int64Observer) error {
		obsrv.Observe(0)
		return nil
	}))
	testutil.FatalOnErr("unexpected err on RegisterInt64Counter", errRegister, t)
}
