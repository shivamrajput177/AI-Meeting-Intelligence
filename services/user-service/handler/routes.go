package handler

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
