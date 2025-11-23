package hooks

import (
	"context"
	"testing"
	"time"

	"github.com/jeanmolossi/chizuql"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestMetricsHookRecordsValues(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	hook := &MetricsHook{Meter: provider.Meter("test")}

	result := chizuql.BuildResult{
		SQL:  "SELECT 1",
		Args: []any{1, "a"},
		Report: chizuql.BuildReport{
			RenderDuration: 15 * time.Millisecond,
			ArgsCount:      2,
			DialectKind:    "postgres",
		},
	}

	if err := hook.AfterBuild(context.Background(), result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect failed: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatalf("expected scope metrics")
	}

	metrics := rm.ScopeMetrics[0].Metrics
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	var (
		durationFound bool
		argsFound     bool
	)

	for _, metric := range metrics {
		switch data := metric.Data.(type) {
		case metricdata.Histogram[float64]:
			durationFound = true

			if len(data.DataPoints) != 1 {
				t.Fatalf("expected 1 datapoint for duration, got %d", len(data.DataPoints))
			}

			if data.DataPoints[0].Sum != 15 {
				t.Fatalf("unexpected duration sum: %v", data.DataPoints[0].Sum)
			}
		case metricdata.Sum[int64]:
			argsFound = true

			if len(data.DataPoints) != 1 {
				t.Fatalf("expected 1 datapoint for args, got %d", len(data.DataPoints))
			}

			if data.DataPoints[0].Value != 2 {
				t.Fatalf("unexpected args count: %d", data.DataPoints[0].Value)
			}
		}
	}

	if !durationFound {
		t.Fatalf("duration metric not found")
	}

	if !argsFound {
		t.Fatalf("args metric not found")
	}
}
