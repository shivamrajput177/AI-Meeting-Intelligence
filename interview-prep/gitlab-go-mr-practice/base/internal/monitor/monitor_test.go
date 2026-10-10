package monitor

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func TestMonitor_Snapshot(t *testing.T) {
	srv := newServer(t, http.StatusOK, 0)
	eps := []Endpoint{{Name: "a", URL: srv.URL}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	m := NewMonitor(NewChecker(time.Second), eps, time.Hour, logger)
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
