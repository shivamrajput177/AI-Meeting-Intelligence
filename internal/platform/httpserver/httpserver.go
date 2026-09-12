// Package httpserver is the one Fiber bootstrap every service (gateway
// included) uses, so the middleware chain — request id, structured
// logging, panic recovery, error formatting — is identical everywhere,
// per docs/architecture/microservices.md §"Internal Communication".
package httpserver

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
)

// New returns a Fiber app with the shared middleware chain installed.
func New(serviceName string, log *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: apperr.FiberHandler,
		AppName:      serviceName,
	})

	app.Use(requestid.New(requestid.Config{
		Header:     reqctx.HeaderRequestID,
		Generator:  func() string { return uuid.NewString() },
		ContextKey: "requestid",
	}))
	app.Use(recover.New())
	app.Use(accessLog(log))
	app.Use(contextFromHeaders())

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/readyz", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	return app
}

// accessLog emits one structured log line per request — method, path,
// status, duration, request id — the minimum RED-metric-adjacent
// information to debug anything in Phase 1 before Prometheus/Grafana
// exist (that's Phase 6).
func accessLog(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		log.Info("http_request",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", fmt.Sprint(c.Locals("requestid"))),
		)
		return err
	}
}
