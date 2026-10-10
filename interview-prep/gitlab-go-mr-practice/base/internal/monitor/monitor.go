package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Monitor periodically checks a set of endpoints and keeps the latest
// results in memory.
type Monitor struct {
	checker   *Checker
	endpoints []Endpoint
	interval  time.Duration
	logger    *slog.Logger

	mu   sync.RWMutex
	last []Result
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
// ctx is cancelled.
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

func (m *Monitor) runOnce(ctx context.Context) {
	results := m.checker.CheckAll(ctx, m.endpoints)

	for _, r := range results {
		if !r.Healthy() {
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
