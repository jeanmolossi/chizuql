package hooks

import (
	"context"
	"sync"

	"github.com/jeanmolossi/chizuql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsHook reports render timing and argument usage to OpenTelemetry metrics.
//
// Provide a Meter to bind the instruments to a specific MeterProvider. When not
// provided, the global MeterProvider is used.
type MetricsHook struct {
	Meter metric.Meter

	once      sync.Once
	duration  metric.Float64Histogram
	argsCount metric.Int64Counter
	initErr   error
}

func (h *MetricsHook) meter() metric.Meter {
	if h.Meter != nil {
		return h.Meter
	}

	return otel.Meter("github.com/jeanmolossi/chizuql/hooks/metrics")
}

func (h *MetricsHook) initInstruments() {
	h.once.Do(func() {
		h.duration, h.initErr = h.meter().Float64Histogram("chizuql.build.duration_ms",
			metric.WithUnit("ms"),
			metric.WithDescription("Render duration for chizuql query builds"),
		)
		if h.initErr != nil {
			return
		}

		h.argsCount, h.initErr = h.meter().Int64Counter("chizuql.build.args",
			metric.WithUnit("{arguments}"),
			metric.WithDescription("Argument placeholders emitted during query build"),
		)
	})
}

// BeforeBuild is a no-op for metrics collection.
func (h *MetricsHook) BeforeBuild(context.Context, *chizuql.Query) error { return nil }

// AfterBuild records render metrics using the configured instruments.
func (h *MetricsHook) AfterBuild(ctx context.Context, result chizuql.BuildResult) error {
	h.initInstruments()

	if h.initErr != nil {
		return h.initErr
	}

	attrs := []attribute.KeyValue{
		attribute.String("db.system", string(result.Report.DialectKind)),
	}

	h.duration.Record(ctx, float64(result.Report.RenderDuration.Milliseconds()), metric.WithAttributes(attrs...))
	h.argsCount.Add(ctx, int64(result.Report.ArgsCount), metric.WithAttributes(attrs...))

	return nil
}
