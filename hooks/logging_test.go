package hooks

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jeanmolossi/chizuql"
)

type recordingHandler struct {
	mu      sync.Mutex
	attrs   []slog.Attr
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	r = r.Clone()
	for _, attr := range h.attrs {
		r.AddAttrs(attr)
	}

	h.records = append(h.records, r)

	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := &recordingHandler{}
	clone.attrs = append(clone.attrs, h.attrs...)
	clone.attrs = append(clone.attrs, attrs...)

	return clone
}

func (h *recordingHandler) WithGroup(string) slog.Handler { return h }

func TestLoggingHookLogsAttributes(t *testing.T) {
	handler := &recordingHandler{}
	logger := slog.New(handler)
	hook := LoggingHook{Logger: logger, IncludeSQL: true}

	result := chizuql.BuildResult{
		SQL:  "SELECT 1",
		Args: []any{1},
		Report: chizuql.BuildReport{
			RenderDuration: 8 * time.Millisecond,
			ArgsCount:      1,
			DialectKind:    "mysql",
		},
	}

	if err := hook.AfterBuild(context.Background(), result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.records) != 1 {
		t.Fatalf("expected one log record, got %d", len(handler.records))
	}

	record := handler.records[0]

	var seenSQL bool

	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "sql" {
			seenSQL = true
		}

		return true
	})

	if !seenSQL {
		t.Fatalf("expected SQL to be logged")
	}
}
