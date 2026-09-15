// Package routes mounts Organization Service's handler.Handler methods
// onto a *http.ServeMux — kept separate from package handler so "what
// each route does" (handler.go) and "which path/method maps to which
// method" (this file) are two files you can read independently.
package routes

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/handler"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

// RegisterRoutes mounts Organization Service's routes on mux.
// internalToken guards the /internal/* routes only — GetOrg is reachable
// both ways (see its doc comment).
func RegisterRoutes(mux *http.ServeMux, h *handler.Handler, internalToken string) {
	mux.Handle("POST /internal/orgs", middleware.RequireInternalToken(internalToken, httpserver.H(h.CreateOrg)))

	mux.Handle("GET /orgs/{orgId}", httpserver.H(h.GetOrg))
}
