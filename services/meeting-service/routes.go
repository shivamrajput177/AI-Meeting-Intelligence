// routes.go mounts Meeting Service's Handler methods onto a
// *http.ServeMux — kept as its own file, not folded into handler.go, so
// "what each route does" (handler.go) and "which path/method maps to
// which method" (this file) stay two things you can read independently
// even though they're one package now.
package main

import (
	"net/http"

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
	mux.HandleFunc("POST /meetings", h.CreateUploadIntent)
	mux.HandleFunc("POST /meetings/{id}/complete-upload", h.ConfirmUpload)
	mux.HandleFunc("GET /meetings/{id}", h.GetMeeting)
	mux.HandleFunc("GET /meetings/{id}/status", h.GetStatus)
	mux.HandleFunc("PATCH /meetings/{id}/status", h.UpdateStatus)
	mux.HandleFunc("GET /meetings", h.ListMeetings)
	mux.HandleFunc("DELETE /meetings/{id}", h.DeleteMeeting)

	mux.Handle("GET /internal/meetings/{id}/participants",
		middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.GetParticipantsInternal)))
	mux.Handle("GET /internal/meetings/{id}",
		middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.GetMeetingInternal)))
	mux.Handle("GET /internal/meetings",
		middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.ListMeetingsInternal)))
}
