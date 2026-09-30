// routes.go mounts User Service's Handler methods onto a *http.ServeMux
// — kept as its own file, not folded into handler.go, so "what each
// route does" (handler.go) and "which path/method maps to which method"
// (this file) stay two things you can read independently even though
// they're one package now.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

// requireOwnerOrAdmin is this service's own re-check of the same
// owner/admin gate the gateway's RequireRole already applied — defense
// in depth per docs/architecture/observability-security.md §2 ("the
// gateway is not trusted as the sole enforcement point").
func requireOwnerOrAdmin(next http.Handler) http.Handler {
	return middleware.RequireRole("owner", "admin")(next)
}

func RegisterRoutes(mux *http.ServeMux, h *Handler, internalToken string) {
	mux.Handle("POST /internal/users", middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.CreateUser)))
	mux.Handle("GET /internal/users/lookup", middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.LookupByEmail)))
	mux.Handle("POST /internal/invites/accept", middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.AcceptInvite)))
	mux.Handle("GET /internal/users/{userId}", middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.GetUserInternal)))

	mux.HandleFunc("GET /users/me", h.GetMe)
	mux.HandleFunc("PATCH /users/me", h.UpdateMe)
	mux.HandleFunc("GET /orgs/{orgId}/users", h.ListUsers)
	mux.HandleFunc("GET /orgs/{orgId}/users/{userId}", h.GetUser)
	mux.Handle("POST /orgs/{orgId}/invites", requireOwnerOrAdmin(http.HandlerFunc(h.CreateInvite)))
	mux.Handle("PATCH /orgs/{orgId}/users/{userId}/role", requireOwnerOrAdmin(http.HandlerFunc(h.UpdateRole)))
	mux.Handle("DELETE /orgs/{orgId}/users/{userId}", requireOwnerOrAdmin(http.HandlerFunc(h.DeactivateUser)))
}
