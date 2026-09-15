// routes.go mounts Organization Service's Handler methods onto a
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

// RegisterRoutes mounts Organization Service's routes on mux.
// internalToken guards the /internal/* routes only — GetOrg is reachable
// both ways (see its doc comment).
func RegisterRoutes(mux *http.ServeMux, h *Handler, internalToken string) {
	mux.Handle("POST /internal/orgs", middleware.RequireInternalToken(internalToken, httpserver.H(h.CreateOrg)))

	mux.Handle("GET /orgs/{orgId}", httpserver.H(h.GetOrg))
}
