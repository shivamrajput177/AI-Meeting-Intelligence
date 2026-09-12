package http

import (
	"github.com/gofiber/fiber/v2"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
)

func RegisterRoutes(app *fiber.App, h *Handler, internalToken string) {
	internal := app.Group("/internal", httpserver.RequireInternalToken(internalToken))
	internal.Post("/users", h.CreateUser)
	internal.Get("/users/lookup", h.LookupByEmail)

	app.Get("/users/me", h.GetMe)
	app.Patch("/users/me", h.UpdateMe)
	app.Get("/orgs/:orgId/users/:userId", h.GetUser)
}
