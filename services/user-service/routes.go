// routes.go mounts User Service's Handler methods onto a *http.ServeMux
// — kept as its own file, not folded into handler.go, so "what each
// route does" (handler.go) and "which path/method maps to which method"
// (this file) stay two things you can read independently even though
// they're one package now.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler, internalToken string) {
	mux.Handle("POST /internal/users", middleware.RequireInternalToken(internalToken, httpserver.H(h.CreateUser)))
	mux.Handle("GET /internal/users/lookup", middleware.RequireInternalToken(internalToken, httpserver.H(h.LookupByEmail)))

	mux.Handle("GET /users/me", httpserver.H(h.GetMe))
	mux.Handle("PATCH /users/me", httpserver.H(h.UpdateMe))
	mux.Handle("GET /orgs/{orgId}/users/{userId}", httpserver.H(h.GetUser))
}
