package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Stats holds counters about the checks performed by a Monitor.
type Stats struct {
	Runs     int `json:"runs"`
	Checks   int `json:"checks"`
	Failures int `json:"failures"`
}

// Monitor periodically checks a set of endpoints and keeps the latest
// results in memory.
type Monitor struct {
	checker   *Checker
	endpoints []Endpoint
	interval  time.Duration
	logger    *slog.Logger

	mu    sync.RWMutex
	last  []Result
	stats Stats
}

// NewMonitor returns a Monitor that checks endpoints every interval.
func NewMonitor(checker *Checker, endpoints []Endpoint, interval time.Duration, logger *slog.Logger) *Monitor {
	return &Monitor{
		checker:   checker,
		endpoints: endpoints,
		interval:  interval,
		logger:    logger,
	}
}

// Run checks the endpoints immediately and then once every interval until
// ctx is cancelled. Runs never overlap: if a run takes longer than interval,
// the ticks missed in the meantime are dropped. Run returns only after the
// run in progress, if any, has finished.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runOnce(ctx)
		}
	}
}

// Snapshot returns a copy of the most recent results.
func (m *Monitor) Snapshot() []Result {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Result, len(m.last))
	copy(out, m.last)

	return out
}

// Stats returns the counters collected so far.
func (m *Monitor) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

func (m *Monitor) runOnce(ctx context.Context) {
	results := m.checker.CheckAll(ctx, m.endpoints)

	// A run interrupted by shutdown would report every endpoint as failed.
	if ctx.Err() != nil {
		return
	}

	failures := 0
	for _, r := range results {
		if !r.Healthy() {
			failures++
			m.logger.Warn("endpoint unhealthy",
				"endpoint", r.Endpoint.Name,
				"status", r.StatusCode,
				"error", r.Error,
			)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.last = results
	m.stats.Runs++
	m.stats.Checks += len(results)
	m.stats.Failures += failures
}
