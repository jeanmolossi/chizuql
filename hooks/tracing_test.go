package hooks

import (
	"context"
	"testing"
	"time"

	"github.com/jeanmolossi/chizuql"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingHookCreatesSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})

	hook := TracingHook{Tracer: tp.Tracer("test"), IncludeSQL: true, SpanName: "build"}

	result := chizuql.BuildResult{
		SQL:  "SELECT 1",
		Args: []any{1},
		Report: chizuql.BuildReport{
			RenderDuration: 5 * time.Millisecond,
			ArgsCount:      1,
			DialectKind:    "mysql",
		},
	}

	if err := hook.AfterBuild(context.Background(), result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name() != "build" {
		t.Fatalf("unexpected span name: %s", span.Name())
	}

	attrs := span.Attributes()
	if len(attrs) == 0 {
		t.Fatalf("expected span attributes")
	}

	foundStatement := false

	for _, attr := range attrs {
		if attr.Key == "db.statement" {
			foundStatement = true
		}
	}

	if !foundStatement {
		t.Fatalf("expected db.statement attribute")
	}
}
