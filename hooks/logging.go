package hooks

import (
	"context"
	"log/slog"

	"github.com/jeanmolossi/chizuql"
)

// LoggingHook emits structured build logs using log/slog.
//
// When Logger is nil, slog.Default is used. Level defaults to slog.LevelInfo.
// Enable IncludeSQL to attach the rendered SQL and arguments.
type LoggingHook struct {
	Logger     *slog.Logger
	Level      slog.Level
	IncludeSQL bool
}

func (h LoggingHook) logger() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}

	return slog.Default()
}

func (h LoggingHook) level() slog.Level {
	if h.Level != 0 {
		return h.Level
	}

	return slog.LevelInfo
}

// BeforeBuild is a no-op for logging.
func (h LoggingHook) BeforeBuild(context.Context, *chizuql.Query) error { return nil }

// AfterBuild logs build metadata and, optionally, the SQL statement.
func (h LoggingHook) AfterBuild(ctx context.Context, result chizuql.BuildResult) error {
	attrs := []slog.Attr{
		slog.String("dialect", string(result.Report.DialectKind)),
		slog.Duration("duration", result.Report.RenderDuration),
		slog.Int("args_count", result.Report.ArgsCount),
	}

	if h.IncludeSQL {
		attrs = append(attrs,
			slog.String("sql", result.SQL),
			slog.Any("args", result.Args),
		)
	}

	h.logger().LogAttrs(ctx, h.level(), "query built", attrs...)

	return nil
}
