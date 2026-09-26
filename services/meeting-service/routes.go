// routes.go mounts Meeting Service's Handler methods onto a
// *http.ServeMux — kept as its own file, not folded into handler.go, so
// "what each route does" (handler.go) and "which path/method maps to
// which method" (this file) stay two things you can read independently
// even though they're one package now.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

// RegisterRoutes mounts Meeting Service's routes. Every public route here
// requires an authenticated caller — the gateway's auth middleware sets
// org/user/role in context before proxying here (see
// services/api-gateway/router.go); this service does not re-verify the JWT
// itself, it trusts the gateway's headers (see reqctx doc comment on the
// trust model this implies). /internal/* routes are gated by
// internalToken instead, for direct service-to-service calls with no JWT.
func RegisterRoutes(mux *http.ServeMux, h *Handler, internalToken string) {
	mux.Handle("POST /meetings", httpserver.H(h.CreateUploadIntent))
	mux.Handle("POST /meetings/{id}/complete-upload", httpserver.H(h.ConfirmUpload))
	mux.Handle("GET /meetings/{id}", httpserver.H(h.GetMeeting))
	mux.Handle("GET /meetings/{id}/status", httpserver.H(h.GetStatus))
	mux.Handle("PATCH /meetings/{id}/status", httpserver.H(h.UpdateStatus))
	mux.Handle("GET /meetings", httpserver.H(h.ListMeetings))
	mux.Handle("DELETE /meetings/{id}", httpserver.H(h.DeleteMeeting))

	mux.Handle("GET /internal/meetings/{id}/participants",
		middleware.RequireInternalToken(internalToken, httpserver.H(h.GetParticipantsInternal)))
}
