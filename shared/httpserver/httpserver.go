// Package httpserver is the one net/http bootstrap every service (gateway
// included) uses, so the middleware chain — request id, logging, panic
// recovery, error formatting — is identical everywhere, per
// docs/architecture/microservices.md §"Internal Communication". Built on
// the standard library only: no web framework, just http.ServeMux (Go
// 1.22+'s method+pattern routing, e.g. "POST /meetings/{id}") wrapped in
// a handful of func(http.Handler) http.Handler middleware from
// shared/middleware.
package httpserver

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/metrics"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

// Server bundles a ServeMux (for route registration) with the shared
// middleware chain already wrapped around it.
type Server struct {
	Mux     *http.ServeMux
	handler http.Handler
}

// New returns a Server with health checks, /metrics, and the shared
// middleware chain installed. Callers register their own routes on
// Mux before calling ListenAndServe. serviceName — unused before Phase 6 —
// now names this process's spans (see otelhttp.NewHandler below); call
// shared/tracing.Init with the same name before New so those spans
// actually export somewhere.
func New(serviceName string, log *logger.Logger) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("GET /metrics", metrics.Handler())

	// Same relative order as before Phase 6 for the original four —
	// withREDMetrics/otelhttp are just inserted as new innermost layers,
	// right around mux, since matching the request's pattern doesn't
	// depend on anything ContextFromHeaders/AccessLog/RecoverPanic/
	// RequestID add.
	var handler http.Handler = mux
	handler = withREDMetrics(mux, handler)
	// otelhttp both starts a span for this request (propagating any
	// inbound W3C traceparent header as its parent — the gateway's own
	// forwarded requests and direct internal calls are traced identically,
	// per observability-security.md §1) and extracts it via
	// shared/httpclient's matching otelhttp.NewTransport on the way back
	// out, which is what actually stitches a multi-service call chain into
	// one trace. WithSpanNameFormatter reuses the same mux-resolved
	// pattern withREDMetrics does, so a span is named "GET /meetings/{id}",
	// not a raw, high-cardinality URL path.
	handler = otelhttp.NewHandler(handler, serviceName, otelhttp.WithSpanNameFormatter(
		func(_ string, r *http.Request) string {
			if _, pattern := mux.Handler(r); pattern != "" {
				return pattern
			}
			return r.Method
		},
	))
	handler = middleware.ContextFromHeaders(handler)
	handler = middleware.AccessLog(log, handler)
	handler = middleware.RecoverPanic(log, handler)
	handler = middleware.RequestID(handler)

	return &Server{Mux: mux, handler: handler}
}

// statusRecorder captures the status code a handler wrote — net/http's
// ResponseWriter has no getter for it. A small, separate copy of the same
// trick shared/middleware's own AccessLog already uses privately; not
// worth exporting just to share five lines across two packages.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// withREDMetrics records shared/metrics' RED metrics for every request —
// this lives in httpserver, not shared/middleware, because it's the only
// place with a *http.ServeMux reference: (*http.ServeMux).Handler(r)
// resolves which registered pattern (e.g. "GET /meetings/{id}") a request
// matched without actually invoking it, which is what keeps the route
// label's cardinality bounded (a raw r.URL.Path would put a different
// label value in Prometheus per meeting id ever requested).
func withREDMetrics(mux *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		_, pattern := mux.Handler(r)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		metrics.ObserveHTTPRequest(r.Method, pattern, strconv.Itoa(rec.status), time.Since(start))
	})
}

// Handler returns the full middleware chain wrapped around Mux — exposed
// so tests can drive a Server end-to-end via httptest without actually
// binding a socket (see httpserver_test.go); ListenAndServe below is the
// only other thing that uses it.
func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.handler)
}
