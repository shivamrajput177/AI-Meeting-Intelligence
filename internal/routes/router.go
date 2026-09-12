// Package routes is the API Gateway's own delivery layer — it has no
// domain/usecase of its own (see docs/architecture/folder-structure.md),
// only routing, auth, and rate-limit middleware in front of a reverse
// proxy to every other service.
package routes

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/routes/middleware"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/routes/proxy"
)

type ServiceURLs struct {
	Auth    string
	User    string
	Org     string
	Meeting string
}

// Register mounts every /api/v1/... route documented in
// docs/architecture/api-spec.md, split into a public group (Auth Service
// — a caller has no JWT yet by definition) and a JWT-protected group
// (everything else).
func Register(app *fiber.App, urls ServiceURLs, jwtSecret []byte, rdb *redis.Client, log *slog.Logger) {
	api := app.Group("/api/v1")

	public := api.Group("")
	public.Post("/auth/signup", middleware.RateLimit(rdb, "auth-signup", 10), proxy.ForwardTo(urls.Auth))
	public.Post("/auth/login", middleware.RateLimit(rdb, "auth-login", 20), proxy.ForwardTo(urls.Auth))
	public.Post("/auth/refresh", proxy.ForwardTo(urls.Auth))
	public.Post("/auth/logout", proxy.ForwardTo(urls.Auth))
	public.Post("/auth/password/reset-request", middleware.RateLimit(rdb, "auth-reset", 10), proxy.ForwardTo(urls.Auth))
	public.Post("/auth/password/reset-confirm", proxy.ForwardTo(urls.Auth))

	protected := api.Group("", middleware.Auth(jwtSecret, rdb, log))

	protected.All("/users/me", proxy.ForwardTo(urls.User))
	protected.All("/orgs/:orgId/users/:userId", proxy.ForwardTo(urls.User))

	protected.All("/orgs", proxy.ForwardTo(urls.Org))
	protected.All("/orgs/:orgId", proxy.ForwardTo(urls.Org))
	protected.All("/orgs/:orgId/settings", proxy.ForwardTo(urls.Org))
	protected.All("/orgs/:orgId/usage", proxy.ForwardTo(urls.Org))

	protected.All("/meetings", proxy.ForwardTo(urls.Meeting))
	protected.All("/meetings/:id", proxy.ForwardTo(urls.Meeting))
	protected.All("/meetings/:id/complete-upload", proxy.ForwardTo(urls.Meeting))
	protected.All("/meetings/:id/status", proxy.ForwardTo(urls.Meeting))
}
