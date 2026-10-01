package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// doRequest drives srv's installed middleware chain directly via
// httptest, without actually binding a port — ListenAndServe itself is
// untested here, same as every other service's own tests never bind a
// real socket.
func doRequest(t *testing.T, srv *httpserver.Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestServer_HealthAndReadyChecks(t *testing.T) {
	srv := httpserver.New("test-service", logger.New("test", logger.LevelError))

	for _, path := range []string{"/healthz", "/readyz"} {
		rec := doRequest(t, srv, "GET", path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", path, rec.Code)
		}
	}
}

func TestServer_MetricsEndpoint_ExposesRouteNotRawPath(t *testing.T) {
	srv := httpserver.New("test-service", logger.New("test", logger.LevelError))
	srv.Mux.HandleFunc("GET /meetings/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Two different ids — if the route label were the raw path instead of
	// the matched pattern, these would produce two distinct label sets
	// instead of one shared "GET /meetings/{id}" series.
	if rec := doRequest(t, srv, "GET", "/meetings/abc123"); rec.Code != http.StatusOK {
		t.Fatalf("GET /meetings/abc123: status = %d, want 200", rec.Code)
	}
	if rec := doRequest(t, srv, "GET", "/meetings/xyz789"); rec.Code != http.StatusOK {
		t.Fatalf("GET /meetings/xyz789: status = %d, want 200", rec.Code)
	}

	rec := doRequest(t, srv, "GET", "/metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics: status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// (*http.ServeMux).Handler returns the pattern exactly as registered,
	// method prefix included ("GET /meetings/{id}"), which is what
	// shared/metrics.ObserveHTTPRequest's route label ends up holding.
	wantLabel := `route="GET /meetings/{id}"`
	if !strings.Contains(body, wantLabel) {
		t.Fatalf("expected /metrics to contain %s, got:\n%s", wantLabel, body)
	}
	if strings.Contains(body, "abc123") || strings.Contains(body, "xyz789") {
		t.Fatalf("expected /metrics never to contain a raw path segment as the route label, got:\n%s", body)
	}
	if !strings.Contains(body, "http_requests_total") || !strings.Contains(body, "http_request_duration_seconds") {
		t.Fatalf("expected both RED metric families present, got:\n%s", body)
	}
}
