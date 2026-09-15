// routes.go mounts Auth Service's Handler methods onto a *http.ServeMux
// — kept as its own file, not folded into handler.go, so "what each
// route does" (handler.go) and "which path/method maps to which method"
// (this file) stay two things you can read independently even though
// they're one package now.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
)

// RegisterRoutes mounts Auth Service's routes. All of them are public —
// the whole point of Auth Service is to be reachable before a caller has
// a JWT — the API Gateway proxies /api/v1/auth/* without its JWT
// middleware (see services/api-gateway/router.go).
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.Handle("POST /auth/signup", httpserver.H(h.Signup))
	mux.Handle("POST /auth/login", httpserver.H(h.Login))
	mux.Handle("POST /auth/refresh", httpserver.H(h.Refresh))
	mux.Handle("POST /auth/logout", httpserver.H(h.Logout))
	mux.Handle("POST /auth/password/reset-request", httpserver.H(h.RequestPasswordReset))
	mux.Handle("POST /auth/password/reset-confirm", httpserver.H(h.ConfirmPasswordReset))
	mux.Handle("POST /invites/{token}/accept", httpserver.H(h.AcceptInvite))
}
