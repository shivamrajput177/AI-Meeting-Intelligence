package monitor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func newServer(t *testing.T, status int, delay time.Duration) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	return srv
}

// waitFor polls cond until it returns true or the deadline expires.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}

	return cond()
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		wantHealthy bool
	}{
		{name: "ok", status: http.StatusOK, wantHealthy: true},
		{name: "redirect", status: http.StatusFound, wantHealthy: true},
		{name: "not found", status: http.StatusNotFound, wantHealthy: false},
		{name: "server error", status: http.StatusInternalServerError, wantHealthy: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newServer(t, tt.status, 0)
			c := NewChecker(time.Second, DefaultConcurrency)

			got := c.Check(context.Background(), Endpoint{Name: tt.name, URL: srv.URL})

			if got.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", got.StatusCode, tt.status)
			}
			if got.Healthy() != tt.wantHealthy {
				t.Errorf("Healthy() = %v, want %v", got.Healthy(), tt.wantHealthy)
			}
		})
	}
}

func TestCheck_Timeout(t *testing.T) {
	srv := newServer(t, http.StatusOK, 200*time.Millisecond)
	c := NewChecker(50*time.Millisecond, DefaultConcurrency)

	got := c.Check(context.Background(), Endpoint{Name: "slow", URL: srv.URL})

	if got.Err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if got.Healthy() {
		t.Error("expected endpoint to be unhealthy")
	}
}

func TestCheck_InvalidURL(t *testing.T) {
	c := NewChecker(time.Second, DefaultConcurrency)

	got := c.Check(context.Background(), Endpoint{Name: "bad", URL: "://not-a-url"})

	if got.Err == nil {
		t.Fatal("expected an error for an invalid URL, got nil")
	}
}

func TestCheckAll_PreservesOrder(t *testing.T) {
	// The first endpoint is the slowest, so with concurrent checks it
	// finishes last. The results must still be in input order.
	slow := newServer(t, http.StatusOK, 100*time.Millisecond)
	broken := newServer(t, http.StatusServiceUnavailable, 0)
	fast := newServer(t, http.StatusOK, 0)

	eps := []Endpoint{
		{Name: "slow", URL: slow.URL},
		{Name: "broken", URL: broken.URL},
		{Name: "fast", URL: fast.URL},
	}

	c := NewChecker(time.Second, len(eps))
	results := c.CheckAll(context.Background(), eps)

	if len(results) != len(eps) {
		t.Fatalf("got %d results, want %d", len(results), len(eps))
	}

	for i, r := range results {
		if r.Endpoint.Name != eps[i].Name {
			t.Errorf("results[%d].Endpoint.Name = %q, want %q", i, r.Endpoint.Name, eps[i].Name)
		}
	}

	if !results[0].Healthy() || results[1].Healthy() || !results[2].Healthy() {
		t.Errorf("unexpected health results: %+v", results)
	}
}

func TestCheckAll_RespectsConcurrencyLimit(t *testing.T) {
	const limit = 3

	var inFlight, maxInFlight, hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		n := inFlight.Add(1)
		defer inFlight.Add(-1)

		for {
			cur := maxInFlight.Load()
			if n <= cur || maxInFlight.CompareAndSwap(cur, n) {
				break
			}
		}

		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	eps := make([]Endpoint, 12)
	for i := range eps {
		eps[i] = Endpoint{Name: fmt.Sprintf("ep-%d", i), URL: srv.URL}
	}

	results := NewChecker(time.Second, limit).CheckAll(context.Background(), eps)

	if len(results) != len(eps) {
		t.Fatalf("got %d results, want %d", len(results), len(eps))
	}
	if got := hits.Load(); got != int32(len(eps)) {
		t.Errorf("server received %d requests, want %d", got, len(eps))
	}
	if got := maxInFlight.Load(); got > limit {
		t.Errorf("max requests in flight = %d, want at most %d", got, limit)
	}
	if got := maxInFlight.Load(); got < 2 {
		t.Errorf("max requests in flight = %d, expected endpoints to be checked concurrently", got)
	}
}

func TestCheckAll_NonPositiveConcurrency(t *testing.T) {
	srv := newServer(t, http.StatusOK, 0)
	eps := []Endpoint{{Name: "a", URL: srv.URL}, {Name: "b", URL: srv.URL}}

	for _, concurrency := range []int{0, -1} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			done := make(chan []Result, 1)
			go func() {
				done <- NewChecker(time.Second, concurrency).CheckAll(context.Background(), eps)
			}()

			select {
			case results := <-done:
				if len(results) != len(eps) {
					t.Errorf("got %d results, want %d", len(results), len(eps))
				}
			case <-time.After(5 * time.Second):
				t.Fatal("CheckAll did not return; it is probably deadlocked")
			}
		})
	}
}

func TestCheckAll_Empty(t *testing.T) {
	results := NewChecker(time.Second, DefaultConcurrency).CheckAll(context.Background(), nil)

	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestFirstFailure(t *testing.T) {
	ok := newServer(t, http.StatusOK, 0)
	broken := newServer(t, http.StatusInternalServerError, 0)

	tests := []struct {
		name string
		eps  []Endpoint
		want string // empty means no failure expected
	}{
		{
			name: "one failure",
			eps:  []Endpoint{{Name: "a", URL: ok.URL}, {Name: "b", URL: broken.URL}, {Name: "c", URL: ok.URL}},
			want: "b",
		},
		{
			name: "all healthy",
			eps:  []Endpoint{{Name: "a", URL: ok.URL}, {Name: "c", URL: ok.URL}},
		},
		{
			name: "no endpoints",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewChecker(time.Second, DefaultConcurrency).FirstFailure(context.Background(), tt.eps)

			switch {
			case tt.want == "" && got != nil:
				t.Errorf("got failure for %q, want none", got.Endpoint.Name)
			case tt.want != "" && got == nil:
				t.Errorf("got no failure, want %q", tt.want)
			case tt.want != "" && got.Endpoint.Name != tt.want:
				t.Errorf("got failure for %q, want %q", got.Endpoint.Name, tt.want)
			}
		})
	}
}

func TestFirstFailure_CancelsRemainingChecksAndLeaksNoGoroutines(t *testing.T) {
	broken := newServer(t, http.StatusInternalServerError, 0)

	// The slow endpoints take far longer than the test allows unless their
	// request is cancelled.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(slow.Close)

	eps := []Endpoint{
		{Name: "slow-1", URL: slow.URL},
		{Name: "broken", URL: broken.URL},
		{Name: "slow-2", URL: slow.URL},
	}

	before := runtime.NumGoroutine()

	start := time.Now()
	got := NewChecker(30*time.Second, len(eps)).FirstFailure(context.Background(), eps)

	if got == nil || got.Endpoint.Name != "broken" {
		t.Fatalf("got %+v, want failure for %q", got, "broken")
	}
	// FirstFailure waits for its workers, so returning quickly proves that the
	// slow checks were cancelled rather than left to run.
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("FirstFailure took %s; in-flight checks were not cancelled", elapsed)
	}

	// Idle keep-alive connections own goroutines too; close them so that only
	// leaked checker goroutines would remain.
	http.DefaultTransport.(*http.Transport).CloseIdleConnections()
	if !waitFor(t, 5*time.Second, func() bool { return runtime.NumGoroutine() <= before }) {
		t.Errorf("goroutines: before=%d after=%d; FirstFailure leaked goroutines", before, runtime.NumGoroutine())
	}
}
