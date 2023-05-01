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
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	ometric "go.opentelemetry.io/otel/metric"
)

// OtelRecorder is a struct used for recording metrics with otel
// It implements MetricsRecorder
type OtelRecorder struct {
	prefix string
	Meter  metric.Meter

	// type: map[string]Int64Counter
	Int64Counters sync.Map
	// type: map[string]Int64Gauge
	Int64Gauges sync.Map
}

type Option interface {
	apply(*OtelRecorder) error
}

type optionFunc func(*OtelRecorder) error

func (o optionFunc) apply(m *OtelRecorder) error {
	return o(m)
}

// addPrefix returns prefix + "_" + name if prefix is not empty
// otherwise, it returns unaltered name
func addPrefix(prefix, name string) string {
	if prefix != "" {
		name = fmt.Sprintf("%s_%s", prefix, name)
	}
	return name
}

// WithMetricNamePrefix adds metric name prefix to the OtelRecorder
func WithMetricNamePrefix(prefix string) Option {
	return optionFunc(func(m *OtelRecorder) error {
		m.prefix = prefix
		return nil
	})
}

// NewOtelRecorder returns a new OtelRecorder instance
func NewOtelRecorder(meter metric.Meter, opts ...Option) (*OtelRecorder, error) {
	m := &OtelRecorder{
		Meter:         meter,
		Int64Counters: sync.Map{},
		Int64Gauges:   sync.Map{},
	}
	for _, o := range opts {
		if err := o.apply(m); err != nil {
			return nil, errors.Wrap(err, "failed to apply option")
		}
	}
	return m, nil
}

// Counter registers the counter if it's not registered, then increments it
func (m *OtelRecorder) Counter(ctx context.Context, metric MetricDefinition, value int64, attributes ...attribute.KeyValue) error {
	metric.Name = addPrefix(m.prefix, metric.Name)
	if counter, exists := m.Int64Counters.Load(metric.Name); exists {
		counter.(ometric.Int64Counter).Add(ctx, value, ometric.WithAttributes(attributes...))
		return nil
	}

	counter, err := m.Meter.Int64Counter(metric.Name, ometric.WithDescription(metric.Description))
	if err != nil {
		return errors.Wrap(err, "failed to create Int64counter")
	}
	counter.Add(ctx, value, ometric.WithAttributes(attributes...))

	m.Int64Counters.Store(metric.Name, counter)
	return nil
}

// Counter registers the counter if it's not registered, then increments it
func (m *OtelRecorder) CounterOrLog(ctx context.Context, metric MetricDefinition, value int64, attributes ...attribute.KeyValue) {
	err := m.Counter(ctx, metric, value, attributes...)
	if err != nil {
		logger := logr.FromContextOrDiscard(ctx)
		logger.Error(err, "unable to record counter")
	}
}

// Gauge registers the gauge along with the callback function that updates its value
func (m *OtelRecorder) Gauge(ctx context.Context, metric MetricDefinition, callback ometric.Int64Callback, attributes ...attribute.KeyValue) error {
	metric.Name = addPrefix(m.prefix, metric.Name)
	gauge, err := m.Meter.Int64ObservableGauge(metric.Name, ometric.WithDescription(metric.Description), ometric.WithInt64Callback(callback))
	if err != nil {
		return err
	}

	m.Int64Gauges.Store(metric.Name, gauge)
	return nil
}

// Counter registers the counter if it's not registered, then increments it
func (m *OtelRecorder) GaugeOrLog(ctx context.Context, metric MetricDefinition, callback ometric.Int64Callback, attributes ...attribute.KeyValue) {
	err := m.Gauge(ctx, metric, callback, attributes...)
	if err != nil {
		logger := logr.FromContextOrDiscard(ctx)
		logger.Error(err, "unable to record gauge")
	}
}
