package metrics

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

/*
const (
	authzDeniedJustificationMetricName = "authz_denied_justification"
	authzDeniedPolicyMetricName        = "authz_denied_policy"
	authzDenialHintErrorMetricName     = "authz_denial_hint_error"
	authzFailureInputMissingMetricName = "authz_failure_input_missing"
	authzFailureEvalErrorMetricName    = "authz_failure_eval_error"
)*/

var (
	m *Metrics
)

type Metrics struct {
	enabled       bool
	prefix        string
	Meter         metric.Meter
	Int64Counters sync.Map
	Int64Gauges   sync.Map
}

func Enabled() bool {
	return m.enabled
}

type Option interface {
	apply(*Metrics) error
}

type optionFunc func(*Metrics) error

func (o optionFunc) apply(m *Metrics) error {
	return o(m)
}

func WithMetricNamePrefix(prefix string) Option {
	return optionFunc(func(m *Metrics) error {
		m.prefix = prefix
		return nil
	})
}

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
			return err
		}
	}
	return nil
}

func RegisterInt64Counter(name, description string) error {
	if !Enabled() {
		return errors.New("metrics is not initialized")
	}
	name = fmt.Sprintf("%s_%s", m.prefix, name)
	if _, exists := m.Int64Counters.Load(name); exists {
		return nil
	}

	counter, err := m.Meter.Int64Counter(name, instrument.WithDescription(description))
	if err != nil {
		return err
	}

	m.Int64Counters.Store(name, counter)
	return nil
}

func AddCount(ctx context.Context, name string, value int64, attributes ...attribute.KeyValue) error {
	name = fmt.Sprintf("%s_%s", m.prefix, name)
	counter, exists := m.Int64Counters.Load(name)
	if !exists {
		return errors.New("counter " + name + " doesn't exist")
	}
	counter.(instrument.Int64Counter).Add(ctx, value, attributes...)

	return nil
}

func RegisterInt64Gauge(name, description string, callback instrument.Int64Callback) error {
	name = fmt.Sprintf("%s_%s", m.prefix, name)
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
