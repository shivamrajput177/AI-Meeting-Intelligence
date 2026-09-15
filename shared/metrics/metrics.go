// Package metrics is a placeholder seam for Phase 6's observability work
// (see docs/architecture/observability-security.md) — this repo has no
// Prometheus/OpenTelemetry wiring yet, so Recorder exists only so calling
// code can be written against a stable interface now and get real metrics
// later by swapping NoOp for a Prometheus-backed implementation, without
// touching every call site.
package metrics

import "time"

// Recorder is the metrics surface every service will eventually call —
// counters for request/error volume, a histogram for latency. NoOp
// satisfies it today; a Phase 6 Prometheus implementation satisfies it
// later.
type Recorder interface {
	// IncCounter increments a named counter by one, tagged with labels
	// (e.g. IncCounter("http_requests_total", map[string]string{"route":
	// "/meetings", "status": "200"})).
	IncCounter(name string, labels map[string]string)

	// ObserveDuration records a duration against a named histogram (e.g.
	// request latency).
	ObserveDuration(name string, labels map[string]string, d time.Duration)
}

// NoOp is a Recorder that discards everything — the default until Phase 6
// wires up a real backend, so nothing in the codebase has to nil-check a
// *metrics.Recorder before using it.
type NoOp struct{}

func (NoOp) IncCounter(string, map[string]string)                     {}
func (NoOp) ObserveDuration(string, map[string]string, time.Duration) {}
