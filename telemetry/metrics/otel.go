package metrics

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
)

// OtelRecorder is a struct used for recording metrics with otel
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

// WithMetricNamePrefix stores the given prefix to the Metrics singleton
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

// RegisterInt64Counter creates an Int64Counter and saves it to the register.
// If there is an existing counter with the same name, it's a no-op.
func (m *OtelRecorder) RegisterInt64Counter(name, description string) error {
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

// AddInt64Counter increments the counter by the given value
// It will return an error if the counter is not registered
func (m *OtelRecorder) AddInt64Counter(ctx context.Context, name string, value int64, attributes ...attribute.KeyValue) error {
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
func (m *OtelRecorder) RegisterInt64Gauge(name, description string, callback instrument.Int64Callback) error {
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
