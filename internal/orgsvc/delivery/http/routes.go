package http

import (
	"github.com/gofiber/fiber/v2"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
)

// RegisterRoutes mounts Organization Service's routes on app.
// internalToken guards the /internal/* routes only — GetOrg is reachable
// both ways (see its doc comment).
func RegisterRoutes(app *fiber.App, h *Handler, internalToken string) {
	internal := app.Group("/internal", httpserver.RequireInternalToken(internalToken))
	internal.Post("/orgs", h.CreateOrg)

	app.Get("/orgs/:orgId", h.GetOrg)
}
