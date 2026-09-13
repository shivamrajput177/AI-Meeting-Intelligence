package logger

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestNew_LogLevelFromEnv(t *testing.T) {
	tests := []struct {
		envValue string
		want     slog.Level
	}{
		{"", slog.LevelInfo},            // unset -> default
		{"info", slog.LevelInfo},        // lowercase
		{"DEBUG", slog.LevelDebug},      // uppercase
		{"Warn", slog.LevelWarn},        // mixed case
		{"error", slog.LevelError},      //
		{"not-a-level", slog.LevelInfo}, // invalid -> falls back to the zero value (info)
	}

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			if tt.envValue == "" {
				_ = os.Unsetenv("LOG_LEVEL")
			} else {
				t.Setenv("LOG_LEVEL", tt.envValue)
			}

			log := New("test-service")
			ctx := context.Background()

			if !log.Enabled(ctx, tt.want) {
				t.Errorf("LOG_LEVEL=%q: expected level %v to be enabled", tt.envValue, tt.want)
			}
			// Debug should be filtered out unless it's the configured
			// level itself — confirms the handler's threshold was
			// actually set, not left permissive.
			if tt.want != slog.LevelDebug && log.Enabled(ctx, slog.LevelDebug) {
				t.Errorf("LOG_LEVEL=%q: expected Debug to be disabled at level %v", tt.envValue, tt.want)
			}
		})
	}
}
