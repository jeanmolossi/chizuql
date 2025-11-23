package hooks

import (
	"context"
	"time"

	"github.com/jeanmolossi/chizuql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracingHook instruments query builds with OpenTelemetry spans.
//
// The hook records the dialect, render duration and argument count. When IncludeSQL
// is true, the final SQL is attached as the db.statement attribute.
//
// Provide a custom tracer via Tracer when you need to bind to a specific
// TracerProvider (for example, in tests). Otherwise, the global tracer
// registered in the OpenTelemetry SDK is used.
type TracingHook struct {
	Tracer     trace.Tracer
	SpanName   string
	IncludeSQL bool
}

func (h TracingHook) tracer() trace.Tracer {
	if h.Tracer != nil {
		return h.Tracer
	}

	return otel.Tracer("github.com/jeanmolossi/chizuql/hooks/tracing")
}

func (h TracingHook) spanName() string {
	if h.SpanName != "" {
		return h.SpanName
	}

	return "chizuql.build"
}

// BeforeBuild is a no-op for tracing.
func (h TracingHook) BeforeBuild(context.Context, *chizuql.Query) error { return nil }

// AfterBuild creates a span annotating the build metrics and, optionally, the SQL text.
func (h TracingHook) AfterBuild(ctx context.Context, result chizuql.BuildResult) error {
	tracer := h.tracer()
	spanStartedAt := time.Now().Add(-result.Report.RenderDuration)
	_, span := tracer.Start(
		ctx,
		h.spanName(),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithTimestamp(spanStartedAt),
	)
	attrs := []attribute.KeyValue{
		attribute.String("db.system", string(result.Report.DialectKind)),
		attribute.Int("db.sql.parameters", result.Report.ArgsCount),
		attribute.Int64("db.render.duration_ms", result.Report.RenderDuration.Milliseconds()),
	}

	if h.IncludeSQL {
		attrs = append(attrs, attribute.String("db.statement", result.SQL))
	}

	span.SetAttributes(attrs...)
	span.End(trace.WithTimestamp(spanStartedAt.Add(result.Report.RenderDuration)))

	return nil
}
