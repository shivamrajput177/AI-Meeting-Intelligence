// Package logger is a small, dependency-free logger: every service prints
// plain "key=value" lines to stdout built directly with fmt — no log/slog,
// no structured Attr/Handler API, just a string assembled by hand. Kept
// deliberately this simple rather than pulling in slog's type system for
// what amounts to a handful of fields (level, service, msg, and a few
// call-specific key/value pairs like status/duration/err).
package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/config"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func parseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LevelDebug
	case "WARN":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Logger prints one line per call: a timestamp, level, the service name
// it was constructed with, the message, and any additional key/value
// pairs — e.g. Info("starting", "addr", ":8080") prints
// `time=... level=INFO service=auth-service msg="starting" addr=:8080`.
type Logger struct {
	service string
	minimum Level
}

// New returns a Logger tagged with the given service name. The minimum
// level printed is controlled by LOG_LEVEL (debug|info|warn|error),
// default info.
func New(service string) *Logger {
	return &Logger{service: service, minimum: parseLevel(config.Env("LOG_LEVEL", "info"))}
}

func (l *Logger) Debug(msg string, kv ...any) { l.print(LevelDebug, msg, kv...) }
func (l *Logger) Info(msg string, kv ...any)  { l.print(LevelInfo, msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.print(LevelWarn, msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.print(LevelError, msg, kv...) }

// print builds the whole line as one string via fmt.Sprintf/strings.Builder
// — no encoder, no reflection-based formatting beyond %v/%q, just direct
// string assembly. kv is read as alternating key, value, key, value, ...;
// an odd trailing element is printed as "key=!MISSING" rather than
// silently dropped or panicking.
func (l *Logger) print(level Level, msg string, kv ...any) {
	if level < l.minimum {
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "time=%s level=%s service=%s msg=%q",
		time.Now().Format(time.RFC3339), level, l.service, msg)

	for i := 0; i < len(kv); i += 2 {
		key := kv[i]
		if i+1 < len(kv) {
			fmt.Fprintf(&b, " %v=%v", key, kv[i+1])
		} else {
			fmt.Fprintf(&b, " %v=!MISSING", key)
		}
	}

	_, _ = fmt.Fprintln(os.Stdout, b.String())
}
