package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}

var jobIDKey = ctxKey{}

func New(level string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	return slog.New(h)
}

func WithJobID(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, jobIDKey, jobID)
}

func JobID(ctx context.Context) string {
	if v, ok := ctx.Value(jobIDKey).(string); ok {
		return v
	}
	return ""
}

func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if id := JobID(ctx); id != "" {
		return base.With("jobId", id)
	}
	return base
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
