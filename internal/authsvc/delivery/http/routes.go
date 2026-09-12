package http

import "github.com/gofiber/fiber/v2"

// RegisterRoutes mounts Auth Service's routes. All of them are public —
// the whole point of Auth Service is to be reachable before a caller has
// a JWT — the API Gateway proxies /api/v1/auth/* without its JWT
// middleware (see internal/routes/router.go).
func RegisterRoutes(app *fiber.App, h *Handler) {
	app.Post("/auth/signup", h.Signup)
	app.Post("/auth/login", h.Login)
	app.Post("/auth/refresh", h.Refresh)
	app.Post("/auth/logout", h.Logout)
	app.Post("/auth/password/reset-request", h.RequestPasswordReset)
	app.Post("/auth/password/reset-confirm", h.ConfirmPasswordReset)
}
