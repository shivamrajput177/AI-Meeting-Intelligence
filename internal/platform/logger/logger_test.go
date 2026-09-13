package logger

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestLogger_FiltersBelowMinimumLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", WithLevel(LevelWarn), WithWriter(&buf))

	log.Debug("should be filtered")
	log.Info("should be filtered")
	log.Warn("should print")
	log.Error("should print")

	out := buf.String()
	if strings.Contains(out, "should be filtered") {
		t.Errorf("expected Debug/Info to be filtered at LevelWarn, got:\n%s", out)
	}
	if strings.Count(out, "should print") != 2 {
		t.Errorf("expected both Warn and Error lines, got:\n%s", out)
	}
}

func TestLogger_PrintsFields(t *testing.T) {
	var buf bytes.Buffer
	log := New("auth-service", WithWriter(&buf))

	log.Info("starting", "addr", ":8080")

	out := buf.String()
	for _, want := range []string{`level=INFO`, `service=auth-service`, `msg="starting"`, `addr=:8080`} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestLogger_With_PrependsBaselineFields(t *testing.T) {
	var buf bytes.Buffer
	base := New("meeting-service", WithWriter(&buf))
	reqLog := base.With("request_id", "abc-123")

	reqLog.Info("http_request", "status", 200)

	out := buf.String()
	if !strings.Contains(out, "request_id=abc-123") {
		t.Errorf("expected request_id from With() in output, got:\n%s", out)
	}
	if !strings.Contains(out, "status=200") {
		t.Errorf("expected status from the call site in output, got:\n%s", out)
	}

	// With must not mutate the original logger — a second call through
	// base should NOT carry request_id.
	buf.Reset()
	base.Info("unrelated")
	if strings.Contains(buf.String(), "request_id") {
		t.Errorf("expected base logger to be unaffected by With(), got:\n%s", buf.String())
	}
}

func TestLogger_OddTrailingKey(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", WithWriter(&buf))

	log.Info("msg", "orphan_key")

	if !strings.Contains(buf.String(), "orphan_key=!MISSING") {
		t.Errorf("expected orphan_key=!MISSING for an odd trailing key, got:\n%s", buf.String())
	}
}
