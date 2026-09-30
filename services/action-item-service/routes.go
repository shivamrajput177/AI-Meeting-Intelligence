// routes.go mounts Action Item Service's Handler methods onto a
// *http.ServeMux — kept as its own file, not folded into handler.go, so
// "what each route does" (handler.go) and "which path/method maps to
// which method" (this file) stay two things you can read independently
// even though they're one package now. Every public route here requires
// an authenticated caller — the gateway's auth middleware sets org/user/role
// in context before proxying here. /internal/* routes are gated by
// internalToken instead, for direct service-to-service calls with no JWT
// (Analytics Service's own per-meeting rollup reads, and Notification
// Service's Jira-ticket-key/status write-back — see
// UpdateActionItemInternalUseCase's doc comment).
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler, internalToken string) {
	mux.Handle("GET /meetings/{id}/action-items", httpserver.H(h.ListActionItemsForMeeting))
	mux.Handle("GET /action-items", httpserver.H(h.ListActionItems))
	mux.Handle("GET /action-items/{id}", httpserver.H(h.GetActionItem))
	mux.Handle("PATCH /action-items/{id}", httpserver.H(h.UpdateActionItem))
	mux.Handle("POST /action-items/{id}/jira-ticket", httpserver.H(h.RequestJiraTicket))

	mux.Handle("GET /internal/meetings/{id}/action-items",
		middleware.RequireInternalToken(internalToken, httpserver.H(h.ListActionItemsForMeetingInternal)))
	mux.Handle("PATCH /internal/action-items/{id}",
		middleware.RequireInternalToken(internalToken, httpserver.H(h.UpdateActionItemInternal)))
}
