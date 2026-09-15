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

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
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
	handler = middleware.ContextFromHeaders(handler)
	handler = middleware.AccessLog(log, handler)
	handler = middleware.RecoverPanic(log, handler)
	handler = middleware.RequestID(handler)

	return &Server{Mux: mux, handler: handler}
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.handler)
}
