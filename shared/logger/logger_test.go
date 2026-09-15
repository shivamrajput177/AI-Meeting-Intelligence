package logger

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
)

// resetSingleton clears the package-level singleton state so each test can
// exercise New() as if it were the first caller in the process. Real
// callers never do this — main() calls New() exactly once — but tests need
// a clean slate to check what a first call does versus what a second,
// contradicting call is ignored.
func resetSingleton() {
	instance = nil
	once = sync.Once{}
}

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
		if got := ParseLevel(tt.in); got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestNew_ReturnsSameInstanceOnSecondCall(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	first := New("auth-service", LevelDebug)
	second := New("a-different-name", LevelError)

	if first != second {
		t.Fatalf("expected New() to return the same *Logger both times, got %p and %p", first, second)
	}
	if second.service != "auth-service" || second.minimum != LevelDebug {
		t.Errorf("expected the second call to be ignored and the first call's config to stick, got service=%q minimum=%v",
			second.service, second.minimum)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// whatever was written to it — needed because print() writes straight to
// os.Stdout rather than through an injectable writer.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestLogger_FiltersBelowMinimumLevel(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	log := New("test", LevelWarn)

	out := captureStdout(t, func() {
		log.Debug("should be filtered")
		log.Info("should be filtered")
		log.Warn("should print")
		log.Error("should print")
	})

	if strings.Contains(out, "should be filtered") {
		t.Errorf("expected Debug/Info to be filtered at LevelWarn, got:\n%s", out)
	}
	if strings.Count(out, "should print") != 2 {
		t.Errorf("expected both Warn and Error lines, got:\n%s", out)
	}
}

func TestLogger_PrintsFields(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	log := New("auth-service", LevelInfo)

	out := captureStdout(t, func() {
		log.Info("starting", "addr", ":8080")
	})

	for _, want := range []string{`level=INFO`, `service=auth-service`, `msg="starting"`, `addr=:8080`} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestLogger_OddTrailingKey(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	log := New("test", LevelInfo)

	out := captureStdout(t, func() {
		log.Info("msg", "orphan_key")
	})

	if !strings.Contains(out, "orphan_key=!MISSING") {
		t.Errorf("expected orphan_key=!MISSING for an odd trailing key, got:\n%s", out)
	}
}
