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
	stop      chan struct{}

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
		stop:      make(chan struct{}),
	}
}

// Run checks the endpoints immediately and then once every interval until
// ctx is cancelled or Stop is called.
//
// Each run happens in its own goroutine so that a slow endpoint cannot delay
// the next scheduled run.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)

	go m.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stop:
			return
		case <-ticker.C:
			go m.runOnce(ctx)
		}
	}
}

// Stop stops the monitor.
func (m *Monitor) Stop() {
	close(m.stop)
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
	return m.stats
}

func (m *Monitor) runOnce(ctx context.Context) {
	results := m.checker.CheckAll(ctx, m.endpoints)

	m.stats.Runs++
	for _, r := range results {
		m.stats.Checks++
		if !r.Healthy() {
			m.stats.Failures++
			m.logger.Warn("endpoint unhealthy",
				"endpoint", r.Endpoint.Name,
				"status", r.StatusCode,
				"error", r.Error,
			)
		}
	}

	m.mu.Lock()
	m.last = results
	m.mu.Unlock()
}
