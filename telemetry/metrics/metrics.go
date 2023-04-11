package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	otelmetricsdk "go.opentelemetry.io/otel/sdk/metric"
)

const (
	authzDeniedJustificationMetricName = "authz_denied_justification"
	authzDeniedPolicyMetricName        = "authz_denied_policy"
	authzDenialHintErrorMetricName     = "authz_denial_hint_error"
	authzFailureInputMissingMetricName = "authz_failure_input_missing"
	authzFailureEvalErrorMetricName    = "authz_failure_eval_error"
)

var (
	mr MetricsRecorder
)

type MetricsRecorderExtended struct {
	MetricsRecorder
	CloudTagsFailureCounter instrument.Int64Counter
}

type MetricsRecorder struct {
	meter        metric.Meter
	metricPrefix string
	enabled      bool

	// instrumentations
	AuthzDeniedJustificationCounter instrument.Int64Counter
	AuthzDeniedPolicyCounter        instrument.Int64Counter
	AuthzFailureInputMissingCounter instrument.Int64Counter
	AuthzFailureEvalErrorCounter    instrument.Int64Counter
	AuthzDenialHintErrorCounter     instrument.Int64Counter
}

func (m MetricsRecorder) Enabled() bool {
	return mr.enabled
}

func GetRecorder() MetricsRecorder {
	return mr
}

func initInstrumentations() error {
	var err error
	mr.AuthzDeniedJustificationCounter, err = mr.meter.Int64Counter(mr.metricPrefix + "_" + authzDeniedJustificationMetricName)
	if err != nil {
		return err
	}
	mr.AuthzFailureInputMissingCounter, err = mr.meter.Int64Counter(mr.metricPrefix + "_" + authzFailureInputMissingMetricName)
	if err != nil {
		return err
	}
	mr.AuthzFailureEvalErrorCounter, err = mr.meter.Int64Counter(mr.metricPrefix + "_" + authzFailureEvalErrorMetricName)
	if err != nil {
		return err
	}
	mr.AuthzDenialHintErrorCounter, err = mr.meter.Int64Counter(mr.metricPrefix + "_" + authzDenialHintErrorMetricName)
	if err != nil {
		return err
	}
	mr.AuthzDeniedPolicyCounter, err = mr.meter.Int64Counter(mr.metricPrefix + "_" + authzDeniedPolicyMetricName)
	if err != nil {
		return err
	}
	return nil
}

func InitProxyMetrics(ctx context.Context) error {
	// Setup exporter using the default prometheus registry
	exporter, err := prometheus.New()
	if err != nil {
		return err
	}
	global.SetMeterProvider(otelmetricsdk.NewMeterProvider(
		otelmetricsdk.WithReader(exporter),
	))
	meter := global.Meter("sansshell-proxy")
	mr = MetricsRecorder{
		meter:        meter,
		metricPrefix: "sansshell-proxy",
		enabled:      true,
	}
	err = initInstrumentations()
	if err != nil {
		return err
	}
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("initialized proxy metrics")

	return nil
}

func InitServerMetrics(ctx context.Context) error {
	meter := global.Meter("sansshell-server")
	mr = MetricsRecorder{
		meter:        meter,
		metricPrefix: "sansshell-server",
		enabled:      true,
	}
	err := initInstrumentations()
	if err != nil {
		return err
	}
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("initialized server metrics")

	return nil
}
