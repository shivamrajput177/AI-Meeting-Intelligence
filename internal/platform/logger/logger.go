// Package logger provides the one slog setup every service uses, so log
// lines are structured JSON with a consistent shape from day one (see
// docs/architecture/observability-security.md §1 — this is the field set
// the Loki/Grafana correlation story in Phase 6 is built to expect).
package logger

import (
	"log/slog"
	"os"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/config"
)

// New returns a JSON slog.Logger tagged with the given service name.
// Level is controlled by LOG_LEVEL (debug|info|warn|error), default info.
func New(service string) *slog.Logger {
	level := slog.LevelInfo
	switch config.Env("LOG_LEVEL", "info") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler).With(slog.String("service", service))
}
