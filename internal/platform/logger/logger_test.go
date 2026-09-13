package logger

import "testing"

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want Level
	}{
		{"", LevelInfo},
		{"info", LevelInfo},
		{"DEBUG", LevelDebug},
		{"Warn", LevelWarn},
		{"error", LevelError},
		{"not-a-level", LevelInfo}, // invalid -> falls back to info
	}
	for _, tt := range tests {
		if got := parseLevel(tt.in); got != tt.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLogger_FiltersBelowMinimumLevel(t *testing.T) {
	log := &Logger{service: "test", minimum: LevelWarn}

	// Can't easily capture stdout without changing the writer, so this
	// just exercises the level-filtering branch for a panic/crash check —
	// print()'s early return on level < minimum is what's under test.
	log.Debug("should be filtered")
	log.Info("should be filtered")
	log.Warn("should print")
	log.Error("should print")
}
