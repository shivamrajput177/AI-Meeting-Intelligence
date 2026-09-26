// routes.go mounts Action Item Service's Handler methods onto a
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
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.Handle("GET /meetings/{id}/action-items", httpserver.H(h.ListActionItemsForMeeting))
	mux.Handle("GET /action-items", httpserver.H(h.ListActionItems))
	mux.Handle("GET /action-items/{id}", httpserver.H(h.GetActionItem))
	mux.Handle("PATCH /action-items/{id}", httpserver.H(h.UpdateActionItem))
}
