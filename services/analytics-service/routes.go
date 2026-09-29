// routes.go mounts Analytics Service's Handler methods onto a
// *http.ServeMux — kept as its own file, not folded into handler.go, so
// "what each route does" (handler.go) and "which path/method maps to
// which method" (this file) stay two things you can read independently
// even though they're one package now. Every route here is "manager+" per
// docs/architecture/api-spec.md's Analytics table — the gateway's own
// RequireRole gate is the other half of this defense-in-depth check (see
// services/api-gateway/router.go), same pattern search-service's
// POST /search/reindex already uses for its own admin-only route. This
// service has no /internal/* routes of its own (nothing else calls it
// service-to-service yet).
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	managerPlus := middleware.RequireRole("owner", "admin", "manager")

	mux.Handle("GET /analytics/meetings/trends", managerPlus(httpserver.H(h.MeetingTrends)))
	mux.Handle("GET /analytics/productivity", managerPlus(httpserver.H(h.Productivity)))
	mux.Handle("GET /analytics/action-items/completion-rate", managerPlus(httpserver.H(h.CompletionRate)))
	mux.Handle("GET /analytics/topics", managerPlus(httpserver.H(h.Topics)))
}
