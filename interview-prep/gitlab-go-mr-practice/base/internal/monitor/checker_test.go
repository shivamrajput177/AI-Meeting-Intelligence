package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
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
			c := NewChecker(time.Second)

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
	c := NewChecker(50 * time.Millisecond)

	got := c.Check(context.Background(), Endpoint{Name: "slow", URL: srv.URL})

	if got.Err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if got.Healthy() {
		t.Error("expected endpoint to be unhealthy")
	}
}

func TestCheck_InvalidURL(t *testing.T) {
	c := NewChecker(time.Second)

	got := c.Check(context.Background(), Endpoint{Name: "bad", URL: "://not-a-url"})

	if got.Err == nil {
		t.Fatal("expected an error for an invalid URL, got nil")
	}
}

func TestCheckAll_PreservesOrder(t *testing.T) {
	ok := newServer(t, http.StatusOK, 0)
	broken := newServer(t, http.StatusServiceUnavailable, 0)

	eps := []Endpoint{
		{Name: "a", URL: ok.URL},
		{Name: "b", URL: broken.URL},
		{Name: "c", URL: ok.URL},
	}

	c := NewChecker(time.Second)
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
