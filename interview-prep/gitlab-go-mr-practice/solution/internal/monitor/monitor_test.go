package monitor

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMonitor_Snapshot(t *testing.T) {
	srv := newServer(t, http.StatusOK, 0)
	eps := []Endpoint{{Name: "a", URL: srv.URL}}

	m := NewMonitor(NewChecker(time.Second, DefaultConcurrency), eps, time.Hour, discardLogger())
	m.runOnce(context.Background())

	snap := m.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("got %d results, want 1", len(snap))
	}
	if !snap[0].Healthy() {
		t.Errorf("expected endpoint to be healthy: %+v", snap[0])
	}

	// Mutating the snapshot must not affect the monitor's internal state.
	snap[0].StatusCode = 0
	if m.Snapshot()[0].StatusCode != http.StatusOK {
		t.Error("Snapshot returned a slice that aliases internal state")
	}
}

func TestMonitor_RunOnceRecordsStats(t *testing.T) {
	ok := newServer(t, http.StatusOK, 0)
	broken := newServer(t, http.StatusInternalServerError, 0)
	eps := []Endpoint{{Name: "a", URL: ok.URL}, {Name: "b", URL: broken.URL}}

	m := NewMonitor(NewChecker(time.Second, DefaultConcurrency), eps, time.Hour, discardLogger())
	m.runOnce(context.Background())
	m.runOnce(context.Background())

	want := Stats{Runs: 2, Checks: 4, Failures: 2}
	if got := m.Stats(); got != want {
		t.Errorf("Stats() = %+v, want %+v", got, want)
	}
}

func TestMonitor_Run(t *testing.T) {
	srv := newServer(t, http.StatusOK, 0)
	eps := []Endpoint{{Name: "a", URL: srv.URL}, {Name: "b", URL: srv.URL}}

	m := NewMonitor(NewChecker(time.Second, 2), eps, 10*time.Millisecond, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Run(ctx)
	}()

	// Read stats concurrently with Run so the race detector can catch
	// unsynchronised access.
	if !waitFor(t, 5*time.Second, func() bool { return m.Stats().Runs >= 3 }) {
		t.Fatalf("expected at least 3 runs, got %+v", m.Stats())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}

	stats := m.Stats()
	if stats.Checks != stats.Runs*len(eps) {
		t.Errorf("Checks = %d, want Runs*len(eps) = %d", stats.Checks, stats.Runs*len(eps))
	}
	if stats.Failures != 0 {
		t.Errorf("Failures = %d, want 0", stats.Failures)
	}

	// After Run has returned no more runs may happen.
	time.Sleep(50 * time.Millisecond)
	if got := m.Stats(); got != stats {
		t.Errorf("stats changed after Run returned: before=%+v after=%+v", stats, got)
	}
}
