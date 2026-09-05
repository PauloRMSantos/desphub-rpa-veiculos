// Package logger configura o slog (stdlib) com saída JSON estruturada.
//
// Regra LGPD: nunca logar PII em claro (CPF, senha, placa completa não devem
// ir para o log). Use os helpers de correlação (jobId) para rastrear execuções.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// ctxKey é o tipo da chave de contexto para o jobId (evita colisão).
type ctxKey struct{}

var jobIDKey = ctxKey{}

// New cria um *slog.Logger com handler JSON no nível informado.
func New(level string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	return slog.New(h)
}

// WithJobID guarda o jobId no contexto para correlação entre etapas.
func WithJobID(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, jobIDKey, jobID)
}

// JobID recupera o jobId do contexto (string vazia se ausente).
func JobID(ctx context.Context) string {
	if v, ok := ctx.Value(jobIDKey).(string); ok {
		return v
	}
	return ""
}

// FromContext devolve um logger já decorado com o jobId do contexto.
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
