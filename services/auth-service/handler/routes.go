package handler

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
)

// RegisterRoutes mounts Auth Service's routes. All of them are public —
// the whole point of Auth Service is to be reachable before a caller has
// a JWT — the API Gateway proxies /api/v1/auth/* without its JWT
// middleware (see services/api-gateway/handler/router.go).
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.Handle("POST /auth/signup", httpserver.H(h.Signup))
	mux.Handle("POST /auth/login", httpserver.H(h.Login))
	mux.Handle("POST /auth/refresh", httpserver.H(h.Refresh))
	mux.Handle("POST /auth/logout", httpserver.H(h.Logout))
	mux.Handle("POST /auth/password/reset-request", httpserver.H(h.RequestPasswordReset))
	mux.Handle("POST /auth/password/reset-confirm", httpserver.H(h.ConfirmPasswordReset))
}
