package logx

import (
	"context"
	"log/slog"
)

type loggerContextKey struct{}

// WithLogger returns a child context carrying logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}

	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// LoggerFromContext returns the logger stored in ctx.
// If ctx does not contain a logger, fallback is returned.
// slog.Default is used when fallback is nil.
func LoggerFromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && logger != nil {
			return logger
		}
	}

	if fallback != nil {
		return fallback
	}

	return slog.Default()
}
