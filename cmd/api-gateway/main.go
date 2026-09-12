// Command api-gateway is the single public entry point — see
// docs/architecture/microservices.md §1.
package main

import (
	"log/slog"
	"os"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/redisx"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/routes"
)

func main() {
	log := logger.New("api-gateway")

	rdb := redisx.NewClient(config.Env("REDIS_ADDR", "localhost:6379"))
	jwtSecret := []byte(config.Env("JWT_SIGNING_KEY", "dev-only-signing-key-change-me"))

	app := httpserver.New("api-gateway", log)
	routes.Register(app, routes.ServiceURLs{
		Auth:    config.Env("AUTH_SERVICE_URL", "http://localhost:8080"),
		User:    config.Env("USER_SERVICE_URL", "http://localhost:8081"),
		Org:     config.Env("ORG_SERVICE_URL", "http://localhost:8082"),
		Meeting: config.Env("MEETING_SERVICE_URL", "http://localhost:8083"),
	}, jwtSecret, rdb, log)

	addr := ":" + config.Env("PORT", "8000")
	log.Info("starting", slog.String("addr", addr))
	if err := app.Listen(addr); err != nil {
		log.Error("server stopped", slog.Any("err", err))
		os.Exit(1)
	}
}
