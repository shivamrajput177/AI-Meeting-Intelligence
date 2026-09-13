package http

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
)

// RegisterRoutes mounts Organization Service's routes on mux.
// internalToken guards the /internal/* routes only — GetOrg is reachable
// both ways (see its doc comment).
func RegisterRoutes(mux *http.ServeMux, h *Handler, internalToken string) {
	mux.Handle("POST /internal/orgs", httpserver.RequireInternalToken(internalToken, httpserver.H(h.CreateOrg)))

	mux.Handle("GET /orgs/{orgId}", httpserver.H(h.GetOrg))
}
