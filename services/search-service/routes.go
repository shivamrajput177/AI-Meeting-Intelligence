// routes.go mounts Search Service's Handler methods onto a
// *http.ServeMux — kept as its own file, not folded into handler.go, so
// "what each route does" (handler.go) and "which path/method maps to
// which method" (this file) stay two things you can read independently
// even though they're one package now. Every route here requires an
// authenticated caller — the gateway's auth middleware sets org/user/role
// in context before proxying here; this service has no /internal/*
// routes of its own (nothing else calls it service-to-service yet).
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.Handle("GET /search", httpserver.H(h.Search))
	mux.Handle("GET /meetings/{id}/similar", httpserver.H(h.SimilarMeetings))
	mux.Handle("POST /qa/ask", httpserver.H(h.Ask))
	mux.Handle("GET /qa/history", httpserver.H(h.GetHistory))

	// admin-only, per docs/architecture/api-spec.md — the gateway's own
	// RequireRole gate is the other half of this defense-in-depth check
	// (see services/api-gateway/router.go).
	mux.Handle("POST /search/reindex", middleware.RequireRole("owner", "admin")(httpserver.H(h.Reindex)))
}
