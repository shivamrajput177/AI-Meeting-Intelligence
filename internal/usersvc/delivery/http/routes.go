package http

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler, internalToken string) {
	mux.Handle("POST /internal/users", httpserver.RequireInternalToken(internalToken, httpserver.H(h.CreateUser)))
	mux.Handle("GET /internal/users/lookup", httpserver.RequireInternalToken(internalToken, httpserver.H(h.LookupByEmail)))

	mux.Handle("GET /users/me", httpserver.H(h.GetMe))
	mux.Handle("PATCH /users/me", httpserver.H(h.UpdateMe))
	mux.Handle("GET /orgs/{orgId}/users/{userId}", httpserver.H(h.GetUser))
}
