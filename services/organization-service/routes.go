// routes.go mounts Organization Service's Handler methods onto a
// *http.ServeMux — kept as its own file, not folded into handler.go, so
// "what each route does" (handler.go) and "which path/method maps to
// which method" (this file) stay two things you can read independently
// even though they're one package now.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

// requireOwnerOrAdmin is this service's own re-check of the same
// owner/admin gate the gateway's RequireRole already applied — defense
// in depth per docs/architecture/observability-security.md §2 ("the
// gateway is not trusted as the sole enforcement point"), the same
// pattern user-service's own routes.go uses.
func requireOwnerOrAdmin(next http.Handler) http.Handler {
	return middleware.RequireRole("owner", "admin")(next)
}

// RegisterRoutes mounts Organization Service's routes on mux.
// internalToken guards the /internal/* routes only — GetOrg is reachable
// both ways (see its doc comment).
func RegisterRoutes(mux *http.ServeMux, h *Handler, internalToken string) {
	mux.Handle("POST /internal/orgs", middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.CreateOrg)))
	mux.Handle("GET /internal/orgs/{orgId}/integration-config",
		middleware.RequireInternalToken(internalToken, http.HandlerFunc(h.GetIntegrationConfigInternal)))

	mux.HandleFunc("GET /orgs/{orgId}", h.GetOrg)
	mux.Handle("PATCH /orgs/{orgId}/settings", requireOwnerOrAdmin(http.HandlerFunc(h.UpdateOrgSettings)))
}
