package http

import "github.com/gofiber/fiber/v2"

// RegisterRoutes mounts Meeting Service's routes. Every one of these
// requires an authenticated caller — the gateway's auth middleware sets
// org/user/role in context before proxying here (see
// internal/routes/router.go); this service does not re-verify the JWT
// itself, it trusts the gateway's headers (see reqctx doc comment on the
// trust model this implies).
func RegisterRoutes(app *fiber.App, h *Handler) {
	app.Post("/meetings", h.CreateUploadIntent)
	app.Post("/meetings/:id/complete-upload", h.ConfirmUpload)
	app.Get("/meetings/:id", h.GetMeeting)
	app.Get("/meetings/:id/status", h.GetStatus)
	app.Patch("/meetings/:id/status", h.UpdateStatus)
	app.Get("/meetings", h.ListMeetings)
	app.Delete("/meetings/:id", h.DeleteMeeting)
}
