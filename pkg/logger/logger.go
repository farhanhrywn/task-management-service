// Package logger configures the JSON slog logger and carries a request-scoped
// logger through context.Context.
package logger

import (
	"context"
	"io"
	"log/slog"
	"time"
)

type ctxKey struct{}

// New returns a JSON logger whose timestamps are UTC with second precision.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				a.Value = slog.StringValue(a.Value.Time().UTC().Truncate(time.Second).Format(time.RFC3339))
			}
			return a
		},
	}))
}

// WithContext stores l in ctx.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the request-scoped logger, or slog.Default().
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
