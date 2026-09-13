// Package logger is a small, dependency-free logger: every service prints
// plain "key=value" lines to stdout built directly with fmt — no log/slog,
// no structured Attr/Handler API, just a string assembled by hand. Kept
// deliberately this simple rather than pulling in slog's type system for
// what amounts to a handful of fields (level, service, msg, and a few
// call-specific key/value pairs like status/duration/err).
//
// This package takes no dependency on internal/platform/config or any
// other package — it reads no environment variables itself. A caller
// (each cmd/*/main.go) reads LOG_LEVEL and passes the parsed Level in via
// WithLevel; logger.New defaults to LevelInfo if that option is omitted.
// Keeping env-var lookups at the edge (main) rather than inside a
// leaf package like this one means logger has exactly one job and can be
// constructed the same way in a test as in production — no hidden global
// config it implicitly reads.
//
// Design note: Logger instances are created once per process (in each
// cmd/*/main.go) and passed explicitly to whatever needs to log —
// constructor injection, not a package-level singleton. That's what makes
// this package's own tests able to construct an isolated *Logger with no
// global state to reset, and it's a deliberate choice, not an oversight:
// see docs/PROJECT_PLAN.md §5's "hand-rolled logger" note for the
// reasoning, and this comment for why Singleton was considered and passed
// over specifically for a logger.
package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
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
	out     io.Writer
	fields  []any // baseline key/value pairs from With(), prepended to every call
}

// Option configures a Logger at construction time — the standard Go
// "functional options" pattern, used here for two knobs (level, output
// destination) rather than growing New's parameter list every time a new
// one shows up.
type Option func(*Logger)

// WithLevel sets the minimum level printed (default LevelInfo). Callers
// pass ParseLevel(config.Env("LOG_LEVEL", "info")) rather than this
// package reading that env var itself — see the package doc comment.
func WithLevel(level Level) Option {
	return func(l *Logger) { l.minimum = level }
}

// WithWriter overrides where log lines are written (default os.Stdout).
// Its reason to exist: tests can pass a *bytes.Buffer to assert on the
// actual line format, which a hardcoded os.Stdout makes impossible.
func WithWriter(w io.Writer) Option {
	return func(l *Logger) { l.out = w }
}

// New returns a Logger tagged with the given service name, defaulting to
// LevelInfo and os.Stdout until overridden by opts.
func New(service string, opts ...Option) *Logger {
	l := &Logger{service: service, minimum: LevelInfo, out: os.Stdout}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// With returns a new Logger that behaves exactly like l, except every
// call on it also prints kv's key/value pairs — a Decorator: it wraps l's
// existing logging behavior with extra baseline context, without either
// mutating l (other holders of it are unaffected) or changing the Logger
// interface callers already use. Typical use: derive one per request
// (log.With("request_id", id)) and pass that down instead of repeating
// the id at every call site.
func (l *Logger) With(kv ...any) *Logger {
	fields := make([]any, 0, len(l.fields)+len(kv))
	fields = append(fields, l.fields...)
	fields = append(fields, kv...)
	return &Logger{service: l.service, minimum: l.minimum, out: l.out, fields: fields}
}

func (l *Logger) Debug(msg string, kv ...any) { l.print(LevelDebug, msg, kv...) }
func (l *Logger) Info(msg string, kv ...any)  { l.print(LevelInfo, msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.print(LevelWarn, msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.print(LevelError, msg, kv...) }

// print builds the whole line as one string via fmt.Sprintf/strings.Builder
// — no encoder, no reflection-based formatting beyond %v/%q, just direct
// string assembly. kv (l.fields followed by this call's own pairs) is read
// as alternating key, value, key, value, ...; an odd trailing element is
// printed as "key=!MISSING" rather than silently dropped or panicking.
func (l *Logger) print(level Level, msg string, kv ...any) {
	if level < l.minimum {
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "time=%s level=%s service=%s msg=%q",
		time.Now().Format(time.RFC3339), level, l.service, msg)

	all := kv
	if len(l.fields) > 0 {
		all = make([]any, 0, len(l.fields)+len(kv))
		all = append(all, l.fields...)
		all = append(all, kv...)
	}

	for i := 0; i < len(all); i += 2 {
		key := all[i]
		if i+1 < len(all) {
			fmt.Fprintf(&b, " %v=%v", key, all[i+1])
		} else {
			fmt.Fprintf(&b, " %v=!MISSING", key)
		}
	}

	_, _ = fmt.Fprintln(l.out, b.String())
}
