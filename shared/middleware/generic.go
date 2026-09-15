package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// RequestID assigns a fresh id to every request (or keeps an
// upstream-supplied one, e.g. from the gateway) and sets it as a response
// header so a client/log line can always be correlated back to this
// request. Every service installs this — see shared/httpserver.New.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(reqctx.HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(reqctx.HeaderRequestID, id)
		r.Header.Set(reqctx.HeaderRequestID, id) // so ContextFromHeaders (next in the chain) picks it up
		next.ServeHTTP(w, r)
	})
}

// RecoverPanic turns a panicking handler into a 500 instead of crashing
// the process — every service needs this exactly once, at the outermost
// layer, not repeated per handler.
func RecoverPanic(log *logger.Logger, next http.Handler) http.Handler {
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

// AccessLog emits one log line per request — method, path, status,
// duration, request id — the minimum RED-metric-adjacent information to
// debug anything in Phase 1 before Prometheus/Grafana exist (that's
// Phase 6 — see shared/metrics for the placeholder that grows into that).
func AccessLog(log *logger.Logger, next http.Handler) http.Handler {
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
