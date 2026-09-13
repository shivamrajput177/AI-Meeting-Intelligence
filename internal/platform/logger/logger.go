// Package logger is a small, dependency-free logger: every service prints
// plain "key=value" lines to stdout built directly with fmt — no log/slog,
// no structured Attr/Handler API, just a string assembled by hand.
//
// Singleton: New checks whether a Logger has already been created for
// this process and returns that one instead of building a second — a
// process here is always exactly one named service logging at one level,
// so there's never a legitimate reason for two different Logger
// configurations to exist side by side. This does not change how callers
// use it: main() still calls New once and passes the *Logger down
// explicitly to whatever needs it (constructor injection), so most code
// never touches the singleton directly — the guard just makes a second,
// inconsistent New(...) call somewhere else in the same process impossible
// instead of silently creating a differently-configured logger.
package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel maps a config string (debug|info|warn|error, case-insensitive)
// to a Level, defaulting to LevelInfo for anything else — including an
// empty or unrecognized string, so a caller can pass a raw env var
// straight through with no separate validation step.
func ParseLevel(s string) Level {
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

var (
	instance *Logger
	once     sync.Once
)

// New returns the single, process-wide Logger. The first call creates it
// with the given service name and level; every later call — even with
// different arguments — returns that same instance rather than building a
// new one. sync.Once (not a plain nil check) makes this safe if New were
// ever called from more than one goroutine, which a plain "if instance ==
// nil" race-condition-checks-and-sets pattern would not be.
func New(service string, level Level) *Logger {
	once.Do(func() {
		instance = &Logger{service: service, minimum: level}
	})
	return instance
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
		if i+1 < len(kv) {
			fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
		} else {
			fmt.Fprintf(&b, " %v=!MISSING", kv[i])
		}
	}

	_, _ = fmt.Fprintln(os.Stdout, b.String())
}
