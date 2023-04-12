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
// package metrics contains code for adding
// metric instrumentations to Sansshell processes
package metrics

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

var (
	// Singelton for instrumenting metrics
	m *Metrics

	errNotInitialized error = errors.New("metrics is not initialized. please call InitMetrics() before anything else")
)

type Metrics struct {
	enabled bool
	prefix  string
	Meter   metric.Meter

	// type: map[string]Int64Counter
	Int64Counters sync.Map
	// type: map[string]Int64Gauge
	Int64Gauges sync.Map
}

type Option interface {
	apply(*Metrics) error
}

type optionFunc func(*Metrics) error

func (o optionFunc) apply(m *Metrics) error {
	return o(m)
}

// addPrefix returns prefix + "_" + name if prefix is not empty
// otherwise, it returns unaltered name
func addPrefix(prefix, name string) string {
	if prefix != "" {
		name = fmt.Sprintf("%s_%s", m.prefix, name)
	}
	return name
}

// WithMetricNamePrefix stores the given prefix to the Metrics singleton
func WithMetricNamePrefix(prefix string) Option {
	return optionFunc(func(m *Metrics) error {
		m.prefix = prefix
		return nil
	})
}

// InitMetrics initializes this package by creating the Metrics singleton
// and applying options.
// IMPORTANT: you need to call this before adding instrumentations
func InitMetrics(opts ...Option) error {
	meter := global.Meter("sansshell-proxy")
	m = &Metrics{
		enabled:       true,
		Meter:         meter,
		Int64Counters: sync.Map{},
		Int64Gauges:   sync.Map{},
	}
	for _, o := range opts {
		if err := o.apply(m); err != nil {
			return errors.Wrap(err, "failed to apply option")
		}
	}
	return nil
}

// Enabled returns true if this package is initialized
func Enabled() bool {
	if m != nil {
		return m.enabled
	}
	return false
}

// RegisterInt64Coungter creates an Int64Counter and saves it to the register.
// If there is an existing counter with the same name, it's a no-op.
func RegisterInt64Counter(name, description string) error {
	if !Enabled() {
		return errNotInitialized
	}
	name = addPrefix(m.prefix, name)
	if _, exists := m.Int64Counters.Load(name); exists {
		return nil
	}

	counter, err := m.Meter.Int64Counter(name, instrument.WithDescription(description))
	if err != nil {
		return errors.Wrap(err, "failed to create Int64counter")
	}

	m.Int64Counters.Store(name, counter)
	return nil
}

// AddCount increments the counter by the given value
// It will return an error if the counter is not registered
func AddCount(ctx context.Context, name string, value int64, attributes ...attribute.KeyValue) error {
	if !Enabled() {
		return errNotInitialized
	}
	name = addPrefix(m.prefix, name)
	counter, exists := m.Int64Counters.Load(name)
	if !exists {
		return errors.New("counter " + name + " doesn't exist")
	}
	counter.(instrument.Int64Counter).Add(ctx, value, attributes...)

	return nil
}

// RegisterInt64Coungter creates an Int64Gauge and saves it to the register.
// If there is an existing counter with the same name, it's a no-op.
func RegisterInt64Gauge(name, description string, callback instrument.Int64Callback) error {
	if !Enabled() {
		return errNotInitialized
	}
	name = addPrefix(m.prefix, name)
	if _, exists := m.Int64Gauges.Load(name); exists {
		return nil
	}
	gauge, err := m.Meter.Int64ObservableGauge(name, instrument.WithDescription(description), instrument.WithInt64Callback(callback))
	if err != nil {
		return err
	}

	m.Int64Gauges.Store(name, gauge)
	return nil
}
