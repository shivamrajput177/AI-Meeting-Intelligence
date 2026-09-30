// routes.go mounts Auth Service's Handler methods onto a *http.ServeMux
// — kept as its own file, not folded into handler.go, so "what each
// route does" (handler.go) and "which path/method maps to which method"
// (this file) stay two things you can read independently even though
// they're one package now.
package main

import (
	"net/http"
)

// RegisterRoutes mounts Auth Service's routes. All of them are public —
// the whole point of Auth Service is to be reachable before a caller has
// a JWT — the API Gateway proxies /api/v1/auth/* without its JWT
// middleware (see services/api-gateway/router.go).
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("POST /auth/signup", h.Signup)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/refresh", h.Refresh)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	mux.HandleFunc("POST /auth/password/reset-request", h.RequestPasswordReset)
	mux.HandleFunc("POST /auth/password/reset-confirm", h.ConfirmPasswordReset)
	mux.HandleFunc("POST /invites/{token}/accept", h.AcceptInvite)
}
