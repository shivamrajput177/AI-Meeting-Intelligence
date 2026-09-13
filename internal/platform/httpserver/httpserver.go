// Package httpserver is the one net/http bootstrap every service (gateway
// included) uses, so the middleware chain — request id, logging, panic
// recovery, error formatting — is identical everywhere, per
// docs/architecture/microservices.md §"Internal Communication". Built on
// the standard library only: no web framework, just http.ServeMux (Go
// 1.22+'s method+pattern routing, e.g. "POST /meetings/{id}") wrapped in
// a handful of func(http.Handler) http.Handler middleware.
package httpserver

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
)

// Server bundles a ServeMux (for route registration) with the shared
// middleware chain already wrapped around it.
type Server struct {
	Mux     *http.ServeMux
	handler http.Handler
}

// New returns a Server with health checks registered and the shared
// middleware chain installed. Callers register their own routes on
// Mux before calling ListenAndServe.
func New(serviceName string, log *logger.Logger) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	var handler http.Handler = mux
	handler = contextFromHeaders(handler)
	handler = accessLog(log, handler)
	handler = recoverPanic(log, handler)
	handler = requestID(handler)

	return &Server{Mux: mux, handler: handler}
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.handler)
}

// requestID assigns a fresh id to every request (or keeps an
// upstream-supplied one, e.g. from the gateway) and sets it as a response
// header so a client/log line can always be correlated back to this
// request.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(reqctx.HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(reqctx.HeaderRequestID, id)
		r.Header.Set(reqctx.HeaderRequestID, id) // so contextFromHeaders (next in the chain) picks it up
		next.ServeHTTP(w, r)
	})
}

// recoverPanic turns a panicking handler into a 500 instead of crashing
// the process — every service needs this exactly once, at the outermost
// layer, not repeated per handler.
func recoverPanic(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered", "panic", rec, "path", r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder wraps a ResponseWriter to capture the status code
// written, since http.ResponseWriter has no getter for it — the standard
// trick for access logging with the stdlib alone.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// accessLog emits one log line per request — method, path, status,
// duration, request id — the minimum RED-metric-adjacent information to
// debug anything in Phase 1 before Prometheus/Grafana exist (that's
// Phase 6).
func accessLog(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
			"request_id", r.Header.Get(reqctx.HeaderRequestID),
		)
	})
}
